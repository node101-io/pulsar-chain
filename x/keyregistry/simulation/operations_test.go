package simulation

import (
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
	"github.com/node101-io/mina-signer-go/publickey"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

func TestRandomMinaPublicKeyLength(t *testing.T) {
	r := rand.New(rand.NewSource(1))

	minaPublicKey := randomMinaPublicKey(r)

	require.Len(t, minaPublicKey, publickey.Size())
}

func TestBuildRegisterKeysMsgUsesUserSimulationAccountPublicKey(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	ctx, k := initSimulationKeeperFixture(t)
	simAccount := simtypes.RandomAccounts(r, 1)[0]

	msg, noOpReason, err := buildRegisterKeysMsg(r, ctx, k, types.ActorType_USER, simAccount)

	require.NoError(t, err)
	require.Empty(t, noOpReason)
	require.Equal(t, simAccount.Address.String(), msg.Creator)
	require.Equal(t, types.ActorType_USER, msg.ActorType)
	require.Equal(t, simAccount.PubKey.Bytes(), msg.CosmosPublicKey)
	require.Len(t, msg.MinaPublicKey, publickey.Size())
	require.NotEmpty(t, msg.CosmosSignature)
	require.NotEmpty(t, msg.MinaSignature)
}

func TestBuildRegisterKeysMsgUsesValidatorConsensusPublicKey(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	ctx, k := initSimulationKeeperFixture(t)
	simAccount := simtypes.RandomAccounts(r, 1)[0]

	msg, noOpReason, err := buildRegisterKeysMsg(r, ctx, k, types.ActorType_VALIDATOR, simAccount)

	require.NoError(t, err)
	require.Empty(t, noOpReason)
	require.Equal(t, simAccount.Address.String(), msg.Creator)
	require.Equal(t, types.ActorType_VALIDATOR, msg.ActorType)
	require.Equal(t, simAccount.ConsKey.PubKey().Bytes(), msg.CosmosPublicKey)
	require.Len(t, msg.CosmosPublicKey, 32)
	require.Len(t, msg.MinaPublicKey, publickey.Size())
	require.NotEmpty(t, msg.CosmosSignature)
	require.NotEmpty(t, msg.MinaSignature)
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
	require.Len(t, minaPublicKey, publickey.Size())
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

func initSimulationKeeperFixture(t *testing.T) (context.Context, keeper.Keeper) {
	t.Helper()

	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
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
