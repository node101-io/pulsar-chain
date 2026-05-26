package simulation

import (
	"context"
	"math/rand"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	simutil "github.com/cosmos/cosmos-sdk/x/simulation"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

const maxUniqueMinaKeyRetries = 20

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
		return types.ActorType_USER
	}

	return types.ActorType_VALIDATOR
}

func randomBytes(r *rand.Rand, size int) []byte {
	bytes := make([]byte, size)
	if _, err := r.Read(bytes); err != nil {
		panic(err)
	}

	return bytes
}

func randomMinaPrivateKey(r *rand.Rand, actorType types.ActorType) (*privatekey.PrivateKey, error) {
	var seed [32]byte
	if _, err := r.Read(seed[:]); err != nil {
		return nil, err
	}

	return privatekey.NewPrivateKeyFromBytes(seed, mina.NetworkID(actorType.String()))
}

func randomMinaKeyPair(r *rand.Rand, actorType types.ActorType) (*privatekey.PrivateKey, []byte, error) {
	minaPrivKey, err := randomMinaPrivateKey(r, actorType)
	if err != nil {
		return nil, nil, err
	}

	minaPubKey, err := minaPrivKey.ToPublicKey()
	if err != nil {
		return nil, nil, err
	}

	return minaPrivKey, minaPubKey.Bytes(), nil
}

func randomMinaPublicKeyForActor(r *rand.Rand, actorType types.ActorType) ([]byte, error) {
	_, minaPubKey, err := randomMinaKeyPair(r, actorType)
	if err != nil {
		return nil, err
	}

	return minaPubKey, nil
}

func randomMinaPublicKey(r *rand.Rand) []byte {
	minaPubKey, err := randomMinaPublicKeyForActor(r, types.ActorType_USER)
	if err != nil {
		panic(err)
	}

	return minaPubKey
}

func signMinaBytes(minaPrivKey *privatekey.PrivateKey, msg []byte) ([]byte, error) {
	minaSig, err := minaPrivKey.SignBytes(msg)
	if err != nil {
		return nil, err
	}

	return minaSig.Bytes(), nil
}

func signCosmosBytesForActor(
	actorType types.ActorType,
	simAccount simtypes.Account,
	msg []byte,
) ([]byte, string, error) {
	switch actorType {
	case types.ActorType_USER:
		if simAccount.PrivKey == nil {
			return nil, "simulation account has no user private key", nil
		}

		sig, err := simAccount.PrivKey.Sign(msg)
		if err != nil {
			return nil, "", err
		}

		return sig, "", nil

	case types.ActorType_VALIDATOR:
		if simAccount.ConsKey == nil {
			return nil, "simulation account has no validator consensus private key", nil
		}

		sig, err := simAccount.ConsKey.Sign(msg)
		if err != nil {
			return nil, "", err
		}

		return sig, "", nil

	default:
		return nil, "invalid actor type", nil
	}
}

func randomUniqueMinaKeyPair(
	r *rand.Rand,
	ctx context.Context,
	actorType types.ActorType,
	hasMinaKey func(context.Context, []byte) (bool, error),
) (*privatekey.PrivateKey, []byte, bool, error) {
	for i := 0; i < maxUniqueMinaKeyRetries; i++ {
		minaPrivKey, minaPubKey, err := randomMinaKeyPair(r, actorType)
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
	r *rand.Rand,
	ctx context.Context,
	hasMinaKey func(context.Context, []byte) (bool, error),
) ([]byte, bool, error) {
	_, minaPubKey, ok, err := randomUniqueMinaKeyPair(r, ctx, types.ActorType_USER, hasMinaKey)
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
) (*types.MsgRegisterKeys, string, error) {
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

	minaPrivKey, minaPublicKey, ok, err := randomUniqueMinaKeyPair(r, ctx, actorType, actorMinaKeyExistsFunc(k, actorType))
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "unable to generate unique mina public key", nil
	}

	cosmosSignature, noOpReason, err := signCosmosBytesForActor(actorType, simAccount, minaPublicKey)
	if err != nil {
		return nil, "", err
	}
	if noOpReason != "" {
		return nil, noOpReason, nil
	}

	minaSignature, err := signMinaBytes(minaPrivKey, cosmosPublicKey)
	if err != nil {
		return nil, "", err
	}

	return &types.MsgRegisterKeys{
		Creator:         simAccount.Address.String(),
		CosmosPublicKey: cosmosPublicKey,
		MinaPublicKey:   minaPublicKey,
		CosmosSignature: cosmosSignature,
		MinaSignature:   minaSignature,
		ActorType:       actorType,
	}, "", nil
}

func buildUpdateKeysMsg(
	r *rand.Rand,
	ctx context.Context,
	k keeper.Keeper,
	actorType types.ActorType,
	genesis *types.GenesisState,
	accs []simtypes.Account,
) (*types.MsgUpdateKeys, simtypes.Account, string, error) {
	var (
		txAccount         simtypes.Account
		signerAccount     simtypes.Account
		cosmosPublicKey   []byte
		prevMinaPublicKey []byte
	)

	switch actorType {
	case types.ActorType_USER:
		userPair, ok := selectRegisteredUserPairWithSigner(r, genesis.UserKeyPairs, accs)
		if !ok {
			return nil, simtypes.Account{}, "no registered user key pair with simulation signer", nil
		}

		txAccount = userPair.account
		signerAccount = userPair.account
		cosmosPublicKey = userPair.pair.CosmosKey
		prevMinaPublicKey = userPair.pair.MinaKey

	case types.ActorType_VALIDATOR:
		validatorPair, ok := selectRegisteredValidatorPairWithSigner(r, genesis.ValidatorKeyPairs, accs)
		if !ok {
			return nil, simtypes.Account{}, "no registered validator key pair with simulation signer", nil
		}

		txAccount = validatorPair.account
		signerAccount = validatorPair.account
		cosmosPublicKey = validatorPair.pair.CosmosKey
		prevMinaPublicKey = validatorPair.pair.MinaKey

	default:
		return nil, simtypes.Account{}, "invalid actor type", nil
	}

	minaPrivKey, newMinaPublicKey, ok, err := randomUniqueMinaKeyPair(r, ctx, actorType, actorMinaKeyExistsFunc(k, actorType))
	if err != nil {
		return nil, simtypes.Account{}, "", err
	}
	if !ok {
		return nil, simtypes.Account{}, "unable to generate unique mina public key", nil
	}

	cosmosSignature, noOpReason, err := signCosmosBytesForActor(actorType, signerAccount, newMinaPublicKey)
	if err != nil {
		return nil, simtypes.Account{}, "", err
	}
	if noOpReason != "" {
		return nil, simtypes.Account{}, noOpReason, nil
	}

	newMinaSignature, err := signMinaBytes(minaPrivKey, cosmosPublicKey)
	if err != nil {
		return nil, simtypes.Account{}, "", err
	}

	return &types.MsgUpdateKeys{
		Creator:           txAccount.Address.String(),
		PrevMinaPublicKey: prevMinaPublicKey,
		NewMinaPublicKey:  newMinaPublicKey,
		CosmosSignature:   cosmosSignature,
		NewMinaSignature:  newMinaSignature,
		ActorType:         actorType,
	}, txAccount, "", nil
}

func cosmosPublicKeyForActor(actorType types.ActorType, simAccount simtypes.Account) ([]byte, string) {
	switch actorType {
	case types.ActorType_USER:
		if simAccount.PubKey == nil {
			return nil, "simulation account has no user public key"
		}

		return simAccount.PubKey.Bytes(), ""

	case types.ActorType_VALIDATOR:
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
	case types.ActorType_USER:
		return k.UserCosmosToMinaHas(ctx, cosmosPublicKey)
	case types.ActorType_VALIDATOR:
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
	case types.ActorType_USER:
		return k.UserMinaToCosmosHas
	case types.ActorType_VALIDATOR:
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
