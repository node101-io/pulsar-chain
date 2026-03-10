package keeper_test

import (
	"context"
	"testing"

	"cosmossdk.io/core/address"
	storetypes "cosmossdk.io/store/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	module "github.com/node101-io/pulsar-chain/x/keyregistry/module"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

type fixture struct {
	ctx          context.Context
	keeper       keeper.Keeper
	addressCodec address.Codec
}

func initFixture(t *testing.T) *fixture {
	t.Helper()

	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx

	authority := authtypes.NewModuleAddress(types.GovModuleName)

	k := keeper.NewKeeper(
		storeService,
		encCfg.Codec,
		addressCodec,
		authority,
	)

	// Initialize params
	if err := k.Params.Set(ctx, types.DefaultParams()); err != nil {
		t.Fatalf("failed to set params: %v", err)
	}

	return &fixture{
		ctx:          ctx,
		keeper:       k,
		addressCodec: addressCodec,
	}
}

// Dummy public keys used in tests.
var CosmosPubKey = []byte("cosmos")
var MinaPubKey = []byte("mina")

// TestCosmosToMina verifies that a cosmos public key can be stored in the
// CosmosToMina map and correctly retrieved using the same cosmos public key.
func TestCosmosToMina(t *testing.T) {
	f := initFixture(t)

	err := f.keeper.SetCosmosToMina(f.ctx, CosmosPubKey, MinaPubKey)
	require.NoError(t, err)

	pubKey, err := f.keeper.GetCosmosToMina(f.ctx, CosmosPubKey)
	require.NoError(t, err)

	require.Equal(t, MinaPubKey, pubKey)
}

// TestMinaToCosmos verifies that a mina public key can be stored in the
// MinaToCosmos map and correctly retrieved using the same mina public key.
func TestMinaToCosmos(t *testing.T) {
	f := initFixture(t)

	err := f.keeper.SetMinaToCosmos(f.ctx, MinaPubKey, CosmosPubKey)
	require.NoError(t, err)

	pubKey, err := f.keeper.GetMinaToCosmos(f.ctx, MinaPubKey)
	require.NoError(t, err)

	require.Equal(t, CosmosPubKey, pubKey)
}
