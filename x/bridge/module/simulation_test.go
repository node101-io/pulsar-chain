package bridge_test

import (
	"encoding/json"
	"testing"

	storetypes "cosmossdk.io/store/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmodule "github.com/cosmos/cosmos-sdk/types/module"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	bridgekeeper "github.com/node101-io/pulsar-chain/x/bridge/keeper"
	bridge "github.com/node101-io/pulsar-chain/x/bridge/module"
	bridgetypes "github.com/node101-io/pulsar-chain/x/bridge/types"
)

func TestGenerateGenesisStateValidateAndInitGenesis(t *testing.T) {
	encCfg := moduletestutil.MakeTestEncodingConfig(bridge.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(bridgetypes.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(bridgetypes.GovModuleName)

	k := bridgekeeper.NewKeeper(
		storeService,
		encCfg.Codec,
		addressCodec,
		authority,
		nil,
		nil,
		nil,
	)

	am := bridge.NewAppModule(encCfg.Codec, k, nil, nil)

	simState := sdkmodule.SimulationState{
		Cdc:      encCfg.Codec,
		GenState: make(map[string]json.RawMessage),
	}

	am.GenerateGenesisState(&simState)

	bz := simState.GenState[bridgetypes.ModuleName]
	require.NotEmpty(t, bz)

	var genState bridgetypes.GenesisState
	require.NoError(t, encCfg.Codec.UnmarshalJSON(bz, &genState))
	require.NoError(t, genState.Validate())
	require.NoError(t, am.ValidateGenesis(encCfg.Codec, nil, bz))

	require.Empty(t, am.WeightedOperations(simState))
	require.Empty(t, am.ProposalMsgs(simState))

	require.NotPanics(t, func() {
		am.InitGenesis(ctx, encCfg.Codec, bz)
	})

	exportedBz := am.ExportGenesis(ctx, encCfg.Codec)

	var exported bridgetypes.GenesisState
	require.NoError(t, encCfg.Codec.UnmarshalJSON(exportedBz, &exported))
	require.EqualExportedValues(t, genState, exported)
}
