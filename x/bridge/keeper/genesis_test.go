package keeper_test

import (
	"testing"

	minafield "github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/pulsar-chain/x/bridge/types"

	"github.com/stretchr/testify/require"
)

func keeperCanonicalRoot(v uint64) []byte {
	return minafield.NewField().FromUint64(v).Bytes()
}

func TestGenesis(t *testing.T) {

	params := types.DefaultTestParams()

	genesisState := types.GenesisState{
		Params:                      params,
		BridgeState:                 types.NewInitialBridgeState(params.StartBlockHeight),
		ActionsReducedRootSnapshots: types.DefaultActionsReducedRootSnapshots(),
	}

	require.NoError(t, genesisState.Validate())

	f := initFixture(t, nil, nil, nil)
	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.EqualExportedValues(t, genesisState.Params, got.Params)
	require.EqualExportedValues(t, genesisState.BridgeState, got.BridgeState)
	require.EqualExportedValues(t, genesisState.ActionsReducedRootSnapshots, got.ActionsReducedRootSnapshots)
}

func TestExportGenesisKeepsRollingWindowOnly(t *testing.T) {
	f := initFixture(t, nil, nil, nil)
	windowSize := validBridgeParams().ActionsReducedRootSnapshotWindowSize

	require.NoError(t, f.keeper.BridgeState.Set(f.ctx, types.DefaultTestBridgeState()))

	for height := int64(1); height <= windowSize+1; height++ {
		require.NoError(t, f.keeper.SetActionsReducedRoot(f.ctx, height, keeperCanonicalRoot(uint64(height))))
	}

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)

	require.Len(t, got.ActionsReducedRootSnapshots, int(windowSize))
	require.Equal(t, int64(2), got.ActionsReducedRootSnapshots[0].CosmosBlockHeight)
	require.Equal(t, int64(5), got.ActionsReducedRootSnapshots[3].CosmosBlockHeight)
	require.Equal(t, keeperCanonicalRoot(5), got.ActionsReducedRootSnapshots[3].ActionsReducedRoot)
	require.NoError(t, got.Validate())
}

func TestGenesisWithCustomStartBlockHeight(t *testing.T) {
	const customStartBlockHeight int64 = 500_000

	genesisState := types.GenesisState{
		Params: types.NewParams(
			testConfirmationDepth,
			testContractAddress,
			customStartBlockHeight,
			testMaxBlockRange,
			testActionsReducedRootSnapshotWindowSize,
		),
		BridgeState:                 types.NewInitialBridgeState(customStartBlockHeight),
		ActionsReducedRootSnapshots: types.DefaultActionsReducedRootSnapshots(),
	}

	f := initFixture(t, nil, nil, nil)
	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.EqualExportedValues(t, genesisState.Params, got.Params)
	require.EqualExportedValues(t, genesisState.BridgeState, got.BridgeState)
	require.EqualExportedValues(t, genesisState.ActionsReducedRootSnapshots, got.ActionsReducedRootSnapshots)
}
