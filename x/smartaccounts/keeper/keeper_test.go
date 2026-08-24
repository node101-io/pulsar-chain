package keeper_test

import (
	"bytes"
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

	"github.com/node101-io/pulsar-chain/x/smartaccounts/keeper"
	module "github.com/node101-io/pulsar-chain/x/smartaccounts/module"
	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"
	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

type mockVerificationKeeper struct {
	result verificationtypes.FinalProofResult
	err    error
}

func (m mockVerificationKeeper) FinalProofResultByProofHash(
	context.Context,
	[]byte,
) (verificationtypes.FinalProofResult, error) {
	return m.result, m.err
}

type fixture struct {
	ctx                context.Context
	keeper             keeper.Keeper
	addressCodec       address.Codec
	verificationKeeper *mockVerificationKeeper
}

func initFixture(t *testing.T) *fixture {
	t.Helper()

	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx

	authority := authtypes.NewModuleAddress(types.GovModuleName)
	verificationKeeper := &mockVerificationKeeper{}

	k := keeper.NewKeeper(
		storeService,
		encCfg.Codec,
		addressCodec,
		authority,
		verificationKeeper,
	)

	// Initialize params
	params := types.NewParams(bytes.Repeat([]byte{0x01}, types.VerificationKeyHashSize))
	if err := k.Params.Set(ctx, params); err != nil {
		t.Fatalf("failed to set params: %v", err)
	}

	return &fixture{
		ctx:                ctx,
		keeper:             k,
		addressCodec:       addressCodec,
		verificationKeeper: verificationKeeper,
	}
}
