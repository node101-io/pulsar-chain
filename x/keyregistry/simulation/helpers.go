package simulation

import (
	"context"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	simutil "github.com/cosmos/cosmos-sdk/x/simulation"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

const maxUniqueMinaKeyRetries = 20

type registeredUserPair struct {
	pair    *types.UserPublicKeyPair
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

func randomMinaPublicKey(r *rand.Rand) []byte {
	// TODO: Generate a real Mina public key once the crypto library is integrated.
	return randomBytes(r, keys.PublicKeyTotalByteSize)
}

func mockSimulationSignature() []byte {
	// TODO: Generate deterministic valid signatures once real crypto verification is integrated.
	return []byte("mock-keyregistry-signature")
}

func randomUniqueMinaPublicKey(
	r *rand.Rand,
	ctx context.Context,
	hasMinaKey func(context.Context, []byte) (bool, error),
) ([]byte, bool, error) {
	for i := 0; i < maxUniqueMinaKeyRetries; i++ {
		minaPublicKey := randomMinaPublicKey(r)
		exists, err := hasMinaKey(ctx, minaPublicKey)
		if err != nil {
			return nil, false, err
		}
		if !exists {
			return minaPublicKey, true, nil
		}
	}

	return nil, false, nil
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

	minaPublicKey, ok, err := randomUniqueMinaPublicKey(r, ctx, actorMinaKeyExistsFunc(k, actorType))
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "unable to generate unique mina public key", nil
	}

	return &types.MsgRegisterKeys{
		Creator:         simAccount.Address.String(),
		CosmosPublicKey: cosmosPublicKey,
		MinaPublicKey:   minaPublicKey,
		CosmosSignature: mockSimulationSignature(),
		MinaSignature:   mockSimulationSignature(),
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
		simAccount        simtypes.Account
		prevMinaPublicKey []byte
		noOpReason        string
	)

	switch actorType {
	case types.ActorType_USER:
		userPair, ok := selectRegisteredUserPairWithSigner(r, genesis.UserKeyPairs, accs)
		if !ok {
			return nil, simtypes.Account{}, "no registered user key pair with simulation signer", nil
		}
		simAccount = userPair.account
		prevMinaPublicKey = userPair.pair.MinaKey
	case types.ActorType_VALIDATOR:
		validatorPair, ok := selectRegisteredValidatorPair(r, genesis.ValidatorKeyPairs)
		if !ok {
			return nil, simtypes.Account{}, "no registered validator key pair", nil
		}
		simAccount, noOpReason = randomSimulationAccount(r, accs)
		if noOpReason != "" {
			return nil, simtypes.Account{}, noOpReason, nil
		}
		prevMinaPublicKey = validatorPair.MinaKey
	default:
		return nil, simtypes.Account{}, "invalid actor type", nil
	}

	newMinaPublicKey, ok, err := randomUniqueMinaPublicKey(r, ctx, actorMinaKeyExistsFunc(k, actorType))
	if err != nil {
		return nil, simtypes.Account{}, "", err
	}
	if !ok {
		return nil, simtypes.Account{}, "unable to generate unique mina public key", nil
	}

	return &types.MsgUpdateKeys{
		Creator:           simAccount.Address.String(),
		PrevMinaPublicKey: prevMinaPublicKey,
		NewMinaPublicKey:  newMinaPublicKey,
		CosmosSignature:   mockSimulationSignature(),
		NewMinaSignature:  mockSimulationSignature(),
		ActorType:         actorType,
	}, simAccount, "", nil
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

func selectRegisteredValidatorPair(
	r *rand.Rand,
	validatorKeyPairs []*types.ValidatorPublicKeyPair,
) (*types.ValidatorPublicKeyPair, bool) {
	var candidates []*types.ValidatorPublicKeyPair
	for _, keyPair := range validatorKeyPairs {
		if keyPair == nil {
			continue
		}
		candidates = append(candidates, keyPair)
	}

	if len(candidates) == 0 {
		return nil, false
	}

	return candidates[r.Intn(len(candidates))], true
}
