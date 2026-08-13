package simulation

import (
	"bytes"
	"context"
	"math/rand"
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

func TestRandomMinaPrivateKeyRetriesInvalidScalar(t *testing.T) {
	reader := bytes.NewReader(append(bytes.Repeat([]byte{0x00}, 32), validSimulationMinaPrivateKeySeed()...))

	privKey, err := randomMinaPrivateKey(reader)

	require.NoError(t, err)
	require.NotNil(t, privKey)
}

func TestRandomMinaPrivateKeyFailsAfterRetries(t *testing.T) {
	reader := bytes.NewReader(bytes.Repeat([]byte{0x00}, maxMinaPrivateKeyRetries*32))

	privKey, err := randomMinaPrivateKey(reader)

	require.Error(t, err)
	require.Nil(t, privKey)
}

func TestRandomMinaPublicKeyIsValid(t *testing.T) {
	r := rand.New(rand.NewSource(1))

	minaPublicKey := randomMinaPublicKey(r)

	require.NoError(t, types.ValidateMinaPublicKey(minaPublicKey))
}

func TestBuildRegisterKeysMsgUsesUserSimulationAccountPublicKey(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	ctx, k := initSimulationKeeperFixture(t)
	simAccount := simtypes.RandomAccounts(r, 1)[0]

	msg, noOpReason, err := buildRegisterKeysMsg(r, ctx, k, types.ActorType_USER, simAccount)

	require.NoError(t, err)
	require.Empty(t, noOpReason)
	userMsg, ok := msg.(*types.MsgRegisterUserKeys)
	require.True(t, ok)
	require.Equal(t, simAccount.Address.String(), userMsg.Creator)
	require.Equal(t, simAccount.PubKey.Bytes(), userMsg.CosmosPublicKey)
	require.NoError(t, types.ValidateMinaPublicKey(userMsg.MinaPublicKey))
	require.NotEmpty(t, userMsg.MinaSignature)

	resp, err := keeper.NewMsgServerImpl(k).RegisterUserKeys(ctx, userMsg)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestBuildRegisterKeysMsgUsesValidatorConsensusPublicKey(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	ctx, k := initSimulationKeeperFixture(t)
	simAccount := simtypes.RandomAccounts(r, 1)[0]

	msg, noOpReason, err := buildRegisterKeysMsg(r, ctx, k, types.ActorType_VALIDATOR, simAccount)

	require.NoError(t, err)
	require.Empty(t, noOpReason)
	validatorMsg, ok := msg.(*types.MsgRegisterValidatorKeys)
	require.True(t, ok)
	require.Equal(t, simAccount.Address.String(), validatorMsg.Creator)
	require.Equal(t, simAccount.ConsKey.PubKey().Bytes(), validatorMsg.ValidatorConsensusPublicKey)
	require.Len(t, validatorMsg.ValidatorConsensusPublicKey, 32)
	require.NoError(t, types.ValidateMinaPublicKey(validatorMsg.MinaPublicKey))
	require.NotEmpty(t, validatorMsg.ValidatorConsensusSignature)
	require.NotEmpty(t, validatorMsg.MinaSignature)

	resp, err := keeper.NewMsgServerImpl(k).RegisterValidatorKeys(ctx, validatorMsg)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestBuildUpdateKeysMsgUsesRegisteredUserPair(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	ctx, k := initSimulationKeeperFixture(t)
	accs := simtypes.RandomAccounts(r, 1)
	genesis := registerSimulationKeyPair(t, r, ctx, k, types.ActorType_USER, accs[0])

	msg, simAccount, noOpReason, err := buildUpdateKeysMsg(r, ctx, k, types.ActorType_USER, genesis, accs)

	require.NoError(t, err)
	require.Empty(t, noOpReason)
	require.True(t, simAccount.Address.Equals(accs[0].Address))
	userMsg, ok := msg.(*types.MsgUpdateUserKeys)
	require.True(t, ok)
	require.NoError(t, types.ValidateMinaPublicKey(userMsg.NewMinaPublicKey))
	require.EqualValues(t, 1, userMsg.NewKeyVersion)
	require.NotEmpty(t, userMsg.NewMinaSignature)

	resp, err := keeper.NewMsgServerImpl(k).UpdateUserKeys(ctx, userMsg)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestBuildUpdateKeysMsgUsesRegisteredValidatorPair(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	ctx, k := initSimulationKeeperFixture(t)
	accs := simtypes.RandomAccounts(r, 1)
	genesis := registerSimulationKeyPair(t, r, ctx, k, types.ActorType_VALIDATOR, accs[0])

	msg, simAccount, noOpReason, err := buildUpdateKeysMsg(r, ctx, k, types.ActorType_VALIDATOR, genesis, accs)

	require.NoError(t, err)
	require.Empty(t, noOpReason)
	require.True(t, simAccount.Address.Equals(accs[0].Address))
	validatorMsg, ok := msg.(*types.MsgUpdateValidatorKeys)
	require.True(t, ok)
	require.NoError(t, types.ValidateMinaPublicKey(validatorMsg.NewMinaPublicKey))
	require.EqualValues(t, 1, validatorMsg.NewKeyVersion)
	require.NotEmpty(t, validatorMsg.ValidatorConsensusSignature)
	require.NotEmpty(t, validatorMsg.NewMinaSignature)

	resp, err := keeper.NewMsgServerImpl(k).UpdateValidatorKeys(ctx, validatorMsg)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestSelectRegisteredUserPairWithSigner(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	accs := simtypes.RandomAccounts(r, 2)
	matchingPair := &types.UserPublicKeyPair{
		CosmosKey: accs[0].PubKey.Bytes(),
		MinaKey:   randomMinaPublicKey(r),
	}
	unmatchedPair := &types.UserPublicKeyPair{
		CosmosKey: randomBytes(r, 33),
		MinaKey:   randomMinaPublicKey(r),
	}

	selected, ok := selectRegisteredUserPairWithSigner(r, []*types.UserPublicKeyPair{
		unmatchedPair,
		matchingPair,
	}, accs)

	require.True(t, ok)
	require.Equal(t, matchingPair, selected.pair)
	require.True(t, selected.account.Address.Equals(accs[0].Address))
}

func TestRandomUniqueMinaPublicKeyRetriesDuplicate(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	calls := 0

	minaPublicKey, ok, err := randomUniqueMinaPublicKey(r, context.Background(), func(context.Context, []byte) (bool, error) {
		calls++
		return calls == 1, nil
	})

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 2, calls)
	require.NoError(t, types.ValidateMinaPublicKey(minaPublicKey))
}

func TestRandomUniqueMinaPublicKeyStopsAfterRetries(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	calls := 0

	minaPublicKey, ok, err := randomUniqueMinaPublicKey(r, context.Background(), func(context.Context, []byte) (bool, error) {
		calls++
		return true, nil
	})

	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, minaPublicKey)
	require.Equal(t, maxUniqueMinaKeyRetries, calls)
}

func registerSimulationKeyPair(
	t *testing.T,
	r *rand.Rand,
	ctx context.Context,
	k keeper.Keeper,
	actorType types.ActorType,
	simAccount simtypes.Account,
) *types.GenesisState {
	t.Helper()

	msg, noOpReason, err := buildRegisterKeysMsg(r, ctx, k, actorType, simAccount)
	require.NoError(t, err)
	require.Empty(t, noOpReason)

	server := keeper.NewMsgServerImpl(k)
	var resp any
	switch typedMsg := msg.(type) {
	case *types.MsgRegisterUserKeys:
		resp, err = server.RegisterUserKeys(ctx, typedMsg)
	case *types.MsgRegisterValidatorKeys:
		resp, err = server.RegisterValidatorKeys(ctx, typedMsg)
	default:
		t.Fatalf("unexpected registration message type %T", msg)
	}
	require.NoError(t, err)
	require.NotNil(t, resp)

	genesis, err := k.ExportGenesis(ctx)
	require.NoError(t, err)

	return genesis
}

func validSimulationMinaPrivateKeySeed() []byte {
	seed := [32]byte([]byte("7olA5Knafb5E2hJoWFzD+oamtyXIXXUZmYG9+pBMjTGIjqZTVLNGbE7DQ3Zq5YL5NMW31UMMMGgNCeEk+gyzRA=="))
	return seed[:]
}

func initSimulationKeeperFixture(t *testing.T) (context.Context, keeper.Keeper) {
	t.Helper()

	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx.WithChainID("pulsar-test-1")
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

	k := keeper.NewKeeper(
		storeService,
		cdc,
		addressCodec,
		authority,
	)

	err := k.Params.Set(ctx, types.DefaultParams())
	require.NoError(t, err)

	return ctx, k
}
