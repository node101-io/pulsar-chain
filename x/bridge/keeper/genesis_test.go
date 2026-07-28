package keeper_test

import (
	"testing"

	"github.com/node101-io/pulsar-chain/x/bridge/types"

	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params:                      types.DefaultParams(),
		BridgeState:                 types.DefaultBridgeState(),
		ActionsReducedRootSnapshots: types.DefaultActionsReducedRootSnapshots(),
	}

	f := initFixture(t)
	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.EqualExportedValues(t, genesisState.Params, got.Params)
	require.EqualExportedValues(t, genesisState.BridgeState, got.BridgeState)
	require.EqualExportedValues(t, genesisState.ActionsReducedRootSnapshots, got.ActionsReducedRootSnapshots)
}

func TestGenesisWithCustomStartBlockHeight(t *testing.T) {
	const customStartBlockHeight int64 = 500_000

	genesisState := types.GenesisState{
		Params: types.NewParams(
			testConfirmationDepth,
			testContractAddress,
			customStartBlockHeight,
		),
		BridgeState:                 types.NewInitialBridgeState(customStartBlockHeight),
		ActionsReducedRootSnapshots: types.DefaultActionsReducedRootSnapshots(),
	}

	f := initFixture(t)
	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.EqualExportedValues(t, genesisState.Params, got.Params)
	require.EqualExportedValues(t, genesisState.BridgeState, got.BridgeState)
	require.EqualExportedValues(t, genesisState.ActionsReducedRootSnapshots, got.ActionsReducedRootSnapshots)
}
