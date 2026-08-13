package simulation

import (
	"context"
	"fmt"
	"io"
	"math/rand"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	simutil "github.com/cosmos/cosmos-sdk/x/simulation"
	minafield "github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

const maxUniqueMinaKeyRetries = 20
const maxMinaPrivateKeyRetries = 100

type registeredUserPair struct {
	pair    *types.UserPublicKeyPair
	account simtypes.Account
}

type registeredValidatorPair struct {
	pair    *types.ValidatorPublicKeyPair
	account simtypes.Account
}

func randomActorType(r *rand.Rand) types.ActorType {
	if r.Intn(2) == 0 {
		return types.ActorType_ACTOR_TYPE_USER
	}

	return types.ActorType_ACTOR_TYPE_VALIDATOR
}

func randomBytes(r *rand.Rand, size int) []byte {
	bytes := make([]byte, size)
	if _, err := r.Read(bytes); err != nil {
		panic(err)
	}

	return bytes
}

func randomMinaPrivateKey(reader io.Reader) (*privatekey.PrivateKey, error) {
	var lastErr error

	for i := 0; i < maxMinaPrivateKeyRetries; i++ {
		var seed [32]byte
		if _, err := io.ReadFull(reader, seed[:]); err != nil {
			return nil, err
		}

		minaPrivKey, err := privatekey.NewPrivateKeyFromBytes(seed, mina.TestNet)
		if err != nil {
			lastErr = err
			continue
		}

		return minaPrivKey, nil
	}

	return nil, fmt.Errorf("unable to generate valid mina private key after %d retries: %w", maxMinaPrivateKeyRetries, lastErr)
}

func randomMinaKeyPair(reader io.Reader) (*privatekey.PrivateKey, []byte, error) {
	minaPrivKey, err := randomMinaPrivateKey(reader)
	if err != nil {
		return nil, nil, err
	}

	minaPubKey, err := minaPrivKey.ToPublicKey()
	if err != nil {
		return nil, nil, err
	}

	return minaPrivKey, minaPubKey.Bytes(), nil
}

func randomMinaPublicKey(reader io.Reader) []byte {
	_, minaPubKey, err := randomMinaKeyPair(reader)
	if err != nil {
		panic(err)
	}

	return minaPubKey
}

func signMinaChallenge(minaPrivKey *privatekey.PrivateKey, challenge *minafield.FieldElement) ([]byte, error) {
	minaSig, err := minaPrivKey.SignFieldElement(challenge)
	if err != nil {
		return nil, err
	}

	return minaSig.Bytes(), nil
}

func randomUniqueMinaKeyPair(
	reader io.Reader,
	ctx context.Context,
	hasMinaKey func(context.Context, []byte) (bool, error),
) (*privatekey.PrivateKey, []byte, bool, error) {
	for i := 0; i < maxUniqueMinaKeyRetries; i++ {
		minaPrivKey, minaPubKey, err := randomMinaKeyPair(reader)
		if err != nil {
			return nil, nil, false, err
		}

		exists, err := hasMinaKey(ctx, minaPubKey)
		if err != nil {
			return nil, nil, false, err
		}

		if !exists {
			return minaPrivKey, minaPubKey, true, nil
		}
	}

	return nil, nil, false, nil
}

func randomUniqueMinaPublicKey(
	reader io.Reader,
	ctx context.Context,
	hasMinaKey func(context.Context, []byte) (bool, error),
) ([]byte, bool, error) {
	_, minaPubKey, ok, err := randomUniqueMinaKeyPair(reader, ctx, hasMinaKey)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	return minaPubKey, true, nil
}

func deliverKeyregistryTx(
	r *rand.Rand,
	app *baseapp.BaseApp,
	txGen client.TxConfig,
	ak types.AuthKeeper,
	bk types.BankKeeper,
	msg sdk.Msg,
	ctx sdk.Context,
	simAccount simtypes.Account,
) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
	if ak.GetAccount(ctx, simAccount.Address) == nil {
		return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "account not found"), nil, nil
	}

	return simutil.GenAndDeliverTxWithRandFees(simutil.OperationInput{
		R:               r,
		App:             app,
		TxGen:           txGen,
		Msg:             msg,
		CoinsSpentInMsg: sdk.Coins{},
		Context:         ctx,
		SimAccount:      simAccount,
		AccountKeeper:   ak,
		Bankkeeper:      bk,
		ModuleName:      types.ModuleName,
	})
}

func buildRegisterKeysMsg(
	r *rand.Rand,
	ctx context.Context,
	k keeper.Keeper,
	actorType types.ActorType,
	simAccount simtypes.Account,
) (sdk.Msg, string, error) {
	cosmosPublicKey, noOpReason := cosmosPublicKeyForActor(actorType, simAccount)
	if noOpReason != "" {
		return nil, noOpReason, nil
	}

	cosmosKeyExists, err := actorCosmosKeyExists(ctx, k, actorType, cosmosPublicKey)
	if err != nil {
		return nil, "", err
	}
	if cosmosKeyExists {
		return nil, "cosmos public key already registered", nil
	}

	minaPrivKey, minaPublicKey, ok, err := randomUniqueMinaKeyPair(r, ctx, actorMinaKeyExistsFunc(k, actorType))
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "unable to generate unique mina public key", nil
	}

	challenge, err := types.BuildKeySigningChallenge(types.KeySigningChallengeInput{
		ChainID:          sdk.UnwrapSDKContext(ctx).ChainID(),
		Operation:        types.KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER,
		ActorType:        actorType,
		CosmosPublicKey:  cosmosPublicKey,
		NewMinaPublicKey: minaPublicKey,
	})
	if err != nil {
		return nil, "", err
	}
	minaSignature, err := signMinaChallenge(minaPrivKey, challenge)
	if err != nil {
		return nil, "", err
	}

	if actorType == types.ActorType_ACTOR_TYPE_USER {
		return &types.MsgRegisterUserKeys{
			Creator:         simAccount.Address.String(),
			CosmosPublicKey: cosmosPublicKey,
			MinaPublicKey:   minaPublicKey,
			MinaSignature:   minaSignature,
		}, "", nil
	}

	consensusSignature, err := simAccount.ConsKey.Sign(challenge.Bytes())
	if err != nil {
		return nil, "", err
	}
	return &types.MsgRegisterValidatorKeys{
		Creator:                     simAccount.Address.String(),
		ValidatorConsensusPublicKey: cosmosPublicKey,
		MinaPublicKey:               minaPublicKey,
		MinaSignature:               minaSignature,
		ValidatorConsensusSignature: consensusSignature,
	}, "", nil
}

func buildUpdateKeysMsg(
	r *rand.Rand,
	ctx context.Context,
	k keeper.Keeper,
	actorType types.ActorType,
	genesis *types.GenesisState,
	accs []simtypes.Account,
) (sdk.Msg, simtypes.Account, string, error) {
	var (
		txAccount            simtypes.Account
		signerAccount        simtypes.Account
		cosmosPublicKey      []byte
		currentMinaPublicKey []byte
		currentKeyVersion    uint64
	)

	switch actorType {
	case types.ActorType_ACTOR_TYPE_USER:
		userPair, ok := selectRegisteredUserPairWithSigner(r, genesis.UserKeyPairs, accs)
		if !ok {
			return nil, simtypes.Account{}, "no registered user key pair with simulation signer", nil
		}

		txAccount = userPair.account
		signerAccount = userPair.account
		cosmosPublicKey = userPair.pair.CosmosKey
		currentMinaPublicKey = userPair.pair.MinaKey
		currentKeyVersion = userPair.pair.KeyVersion

	case types.ActorType_ACTOR_TYPE_VALIDATOR:
		validatorPair, ok := selectRegisteredValidatorPairWithSigner(r, genesis.ValidatorKeyPairs, accs)
		if !ok {
			return nil, simtypes.Account{}, "no registered validator key pair with simulation signer", nil
		}

		txAccount = validatorPair.account
		signerAccount = validatorPair.account
		cosmosPublicKey = validatorPair.pair.CosmosKey
		currentMinaPublicKey = validatorPair.pair.MinaKey
		currentKeyVersion = validatorPair.pair.KeyVersion

	default:
		return nil, simtypes.Account{}, "invalid actor type", nil
	}

	minaPrivKey, newMinaPublicKey, ok, err := randomUniqueMinaKeyPair(r, ctx, actorMinaKeyExistsFunc(k, actorType))
	if err != nil {
		return nil, simtypes.Account{}, "", err
	}
	if !ok {
		return nil, simtypes.Account{}, "unable to generate unique mina public key", nil
	}

	newKeyVersion := currentKeyVersion + 1
	challenge, err := types.BuildKeySigningChallenge(types.KeySigningChallengeInput{
		ChainID:              sdk.UnwrapSDKContext(ctx).ChainID(),
		Operation:            types.KeySigningOperation_KEY_SIGNING_OPERATION_UPDATE,
		ActorType:            actorType,
		CosmosPublicKey:      cosmosPublicKey,
		CurrentMinaPublicKey: currentMinaPublicKey,
		NewMinaPublicKey:     newMinaPublicKey,
		NewKeyVersion:        newKeyVersion,
	})
	if err != nil {
		return nil, simtypes.Account{}, "", err
	}
	newMinaSignature, err := signMinaChallenge(minaPrivKey, challenge)
	if err != nil {
		return nil, simtypes.Account{}, "", err
	}

	if actorType == types.ActorType_ACTOR_TYPE_USER {
		return &types.MsgUpdateUserKeys{
			Creator:          txAccount.Address.String(),
			CosmosPublicKey:  cosmosPublicKey,
			NewMinaPublicKey: newMinaPublicKey,
			NewKeyVersion:    newKeyVersion,
			NewMinaSignature: newMinaSignature,
		}, txAccount, "", nil
	}

	consensusSignature, err := signerAccount.ConsKey.Sign(challenge.Bytes())
	if err != nil {
		return nil, simtypes.Account{}, "", err
	}
	return &types.MsgUpdateValidatorKeys{
		Creator:                     txAccount.Address.String(),
		ValidatorConsensusPublicKey: cosmosPublicKey,
		NewMinaPublicKey:            newMinaPublicKey,
		NewKeyVersion:               newKeyVersion,
		NewMinaSignature:            newMinaSignature,
		ValidatorConsensusSignature: consensusSignature,
	}, txAccount, "", nil
}

func cosmosPublicKeyForActor(actorType types.ActorType, simAccount simtypes.Account) ([]byte, string) {
	switch actorType {
	case types.ActorType_ACTOR_TYPE_USER:
		if simAccount.PubKey == nil {
			return nil, "simulation account has no user public key"
		}

		return simAccount.PubKey.Bytes(), ""

	case types.ActorType_ACTOR_TYPE_VALIDATOR:
		if simAccount.ConsKey == nil {
			return nil, "simulation account has no validator consensus key"
		}

		return simAccount.ConsKey.PubKey().Bytes(), ""

	default:
		return nil, "invalid actor type"
	}
}

func actorCosmosKeyExists(ctx context.Context, k keeper.Keeper, actorType types.ActorType, cosmosPublicKey []byte) (bool, error) {
	switch actorType {
	case types.ActorType_ACTOR_TYPE_USER:
		return k.UserCosmosToMinaHas(ctx, cosmosPublicKey)
	case types.ActorType_ACTOR_TYPE_VALIDATOR:
		return k.ValidatorCosmosToMinaHas(ctx, cosmosPublicKey)
	default:
		return false, types.ErrInvalidActorType
	}
}

func actorMinaKeyExistsFunc(
	k keeper.Keeper,
	actorType types.ActorType,
) func(context.Context, []byte) (bool, error) {
	switch actorType {
	case types.ActorType_ACTOR_TYPE_USER:
		return k.UserMinaToCosmosHas
	case types.ActorType_ACTOR_TYPE_VALIDATOR:
		return k.ValidatorMinaToCosmosHas
	default:
		return func(context.Context, []byte) (bool, error) {
			return false, types.ErrInvalidActorType
		}
	}
}

func randomSimulationAccount(r *rand.Rand, accs []simtypes.Account) (simtypes.Account, string) {
	if len(accs) == 0 {
		return simtypes.Account{}, "no simulation accounts"
	}

	simAccount, _ := simtypes.RandomAcc(r, accs)
	return simAccount, ""
}

func selectRegisteredUserPairWithSigner(
	r *rand.Rand,
	userKeyPairs []*types.UserPublicKeyPair,
	accs []simtypes.Account,
) (registeredUserPair, bool) {
	accountByPublicKey := make(map[string]simtypes.Account, len(accs))
	for _, account := range accs {
		if account.PubKey == nil {
			continue
		}
		accountByPublicKey[string(account.PubKey.Bytes())] = account
	}

	var candidates []registeredUserPair
	for _, keyPair := range userKeyPairs {
		if keyPair == nil {
			continue
		}

		account, ok := accountByPublicKey[string(keyPair.CosmosKey)]
		if !ok {
			continue
		}

		candidates = append(candidates, registeredUserPair{
			pair:    keyPair,
			account: account,
		})
	}

	if len(candidates) == 0 {
		return registeredUserPair{}, false
	}

	return candidates[r.Intn(len(candidates))], true
}

func selectRegisteredValidatorPairWithSigner(
	r *rand.Rand,
	validatorKeyPairs []*types.ValidatorPublicKeyPair,
	accs []simtypes.Account,
) (registeredValidatorPair, bool) {
	accountByPublicKey := make(map[string]simtypes.Account, len(accs))
	for _, account := range accs {
		if account.ConsKey == nil {
			continue
		}
		accountByPublicKey[string(account.ConsKey.PubKey().Bytes())] = account
	}

	var candidates []registeredValidatorPair
	for _, keyPair := range validatorKeyPairs {
		if keyPair == nil {
			continue
		}

		account, ok := accountByPublicKey[string(keyPair.CosmosKey)]
		if !ok {
			continue
		}

		candidates = append(candidates, registeredValidatorPair{
			pair:    keyPair,
			account: account,
		})
	}

	if len(candidates) == 0 {
		return registeredValidatorPair{}, false
	}

	return candidates[r.Intn(len(candidates))], true
}
