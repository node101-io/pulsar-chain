package keeper

import (
	"bytes"
	"context"
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

func TestExportGenesisRejectsInconsistentRuntimeIndexes(t *testing.T) {
	tests := []struct {
		name    string
		corrupt func(context.Context, Keeper) error
	}{
		{
			name: "user forward missing reverse",
			corrupt: func(ctx context.Context, k Keeper) error {
				return k.userCosmosToMina.Set(ctx, testGenesisBytes(33, 'a'), testGenesisBytes(33, 'x'))
			},
		},
		{
			name: "user reverse missing forward",
			corrupt: func(ctx context.Context, k Keeper) error {
				return k.userMinaToCosmos.Set(ctx, testGenesisBytes(33, 'x'), testGenesisBytes(33, 'a'))
			},
		},
		{
			name: "user forward reverse mismatch",
			corrupt: func(ctx context.Context, k Keeper) error {
				if err := k.userCosmosToMina.Set(ctx, testGenesisBytes(33, 'a'), testGenesisBytes(33, 'x')); err != nil {
					return err
				}
				return k.userMinaToCosmos.Set(ctx, testGenesisBytes(33, 'x'), testGenesisBytes(33, 'b'))
			},
		},
		{
			name: "validator forward missing reverse",
			corrupt: func(ctx context.Context, k Keeper) error {
				return k.validatorCosmosToMina.Set(ctx, testGenesisBytes(32, 'a'), testGenesisBytes(33, 'x'))
			},
		},
		{
			name: "validator reverse missing forward",
			corrupt: func(ctx context.Context, k Keeper) error {
				return k.validatorMinaToCosmos.Set(ctx, testGenesisBytes(33, 'x'), testGenesisBytes(32, 'a'))
			},
		},
		{
			name: "validator forward reverse mismatch",
			corrupt: func(ctx context.Context, k Keeper) error {
				if err := k.validatorCosmosToMina.Set(ctx, testGenesisBytes(32, 'a'), testGenesisBytes(33, 'x')); err != nil {
					return err
				}
				return k.validatorMinaToCosmos.Set(ctx, testGenesisBytes(33, 'x'), testGenesisBytes(32, 'b'))
			},
		},
		{
			name: "consistent indexes with invalid exported key length",
			corrupt: func(ctx context.Context, k Keeper) error {
				if err := k.userCosmosToMina.Set(ctx, testGenesisBytes(32, 'a'), testGenesisBytes(33, 'x')); err != nil {
					return err
				}
				return k.userMinaToCosmos.Set(ctx, testGenesisBytes(33, 'x'), testGenesisBytes(32, 'a'))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, k := initInternalGenesisFixture(t)

			err := tc.corrupt(ctx, k)
			require.NoError(t, err)

			genesis, err := k.ExportGenesis(ctx)
			require.Nil(t, genesis)
			require.ErrorIs(t, err, types.ErrInvalidGenesisState)
		})
	}
}

func initInternalGenesisFixture(t *testing.T) (context.Context, Keeper) {
	t.Helper()

	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

	k := NewKeeper(
		storeService,
		cdc,
		addressCodec,
		authority,
	)

	err := k.Params.Set(ctx, types.DefaultParams())
	require.NoError(t, err)

	return ctx, k
}

func testGenesisBytes(length int, value byte) []byte {
	return bytes.Repeat([]byte{value}, length)
}
