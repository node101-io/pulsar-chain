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

func keeperCanonicalActionHash(v uint64) string {
	return minafield.NewField().FromUint64(v).String()
}

func TestGenesis(t *testing.T) {

	params := types.DefaultTestParams()

	genesisState := types.GenesisState{
		Params: params,
		BridgeState: types.BridgeState{
			LatestFetchedMinaHeight: params.StartBlockHeight - 1,
			ValidActionHashes: []string{
				keeperCanonicalActionHash(42),
				keeperCanonicalActionHash(43),
			},
		},
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

func TestPrepareForZeroHeightGenesisKeepsOnlyCurrentRoot(t *testing.T) {
	f := initFixture(t, nil, nil, nil)

	beforeParams, err := f.keeper.Params.Get(f.ctx)
	require.NoError(t, err)
	beforeState := types.BridgeState{
		LatestFetchedMinaHeight: 500_000,
		ValidActionHashes: []string{
			keeperCanonicalActionHash(77),
			keeperCanonicalActionHash(78),
		},
	}
	require.NoError(t, f.keeper.BridgeState.Set(f.ctx, beforeState))

	for height := int64(100); height <= 103; height++ {
		require.NoError(t, f.keeper.SetActionsReducedRoot(f.ctx, height, keeperCanonicalRoot(uint64(height))))
	}

	require.NoError(t, f.keeper.PrepareForZeroHeightGenesis(f.ctx))

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.Len(t, got.ActionsReducedRootSnapshots, 1)
	require.Equal(t, int64(0), got.ActionsReducedRootSnapshots[0].CosmosBlockHeight)
	require.Equal(t, keeperCanonicalRoot(103), got.ActionsReducedRootSnapshots[0].ActionsReducedRoot)
	require.EqualExportedValues(t, beforeParams, got.Params)
	require.EqualExportedValues(t, beforeState, got.BridgeState)

	rootAtZero, err := f.keeper.GetActionsReducedRootAtHeight(f.ctx, 0)
	require.NoError(t, err)
	require.Equal(t, keeperCanonicalRoot(103), rootAtZero)

	latestRoot, err := f.keeper.GetLatestActionsReducedRoot(f.ctx)
	require.NoError(t, err)
	require.Equal(t, keeperCanonicalRoot(103), latestRoot)
}

func TestPrepareForZeroHeightGenesisRestoresSnapshotPruningOrder(t *testing.T) {
	f := initFixture(t, nil, nil, nil)
	require.NoError(t, f.keeper.BridgeState.Set(f.ctx, types.BridgeState{
		LatestFetchedMinaHeight: 500_000,
		ValidActionHashes: []string{
			keeperCanonicalActionHash(90),
		},
	}))

	for height := int64(100); height <= 103; height++ {
		require.NoError(t, f.keeper.SetActionsReducedRoot(f.ctx, height, keeperCanonicalRoot(uint64(height))))
	}
	require.NoError(t, f.keeper.PrepareForZeroHeightGenesis(f.ctx))

	require.NoError(t, f.keeper.SetActionsReducedRoot(f.ctx, 1, keeperCanonicalRoot(201)))
	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.Len(t, got.ActionsReducedRootSnapshots, 2)
	require.Equal(t, int64(0), got.ActionsReducedRootSnapshots[0].CosmosBlockHeight)
	require.Equal(t, int64(1), got.ActionsReducedRootSnapshots[1].CosmosBlockHeight)
	require.Equal(t, keeperCanonicalRoot(201), got.ActionsReducedRootSnapshots[1].ActionsReducedRoot)

	for height := int64(2); height <= 4; height++ {
		require.NoError(t, f.keeper.SetActionsReducedRoot(f.ctx, height, keeperCanonicalRoot(uint64(200+height))))
	}

	got, err = f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.Len(t, got.ActionsReducedRootSnapshots, 4)
	require.Equal(t, []int64{1, 2, 3, 4}, []int64{
		got.ActionsReducedRootSnapshots[0].CosmosBlockHeight,
		got.ActionsReducedRootSnapshots[1].CosmosBlockHeight,
		got.ActionsReducedRootSnapshots[2].CosmosBlockHeight,
		got.ActionsReducedRootSnapshots[3].CosmosBlockHeight,
	})

	latestRoot, err := f.keeper.GetLatestActionsReducedRoot(f.ctx)
	require.NoError(t, err)
	require.Equal(t, keeperCanonicalRoot(204), latestRoot)
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
		BridgeState: types.BridgeState{
			LatestFetchedMinaHeight: customStartBlockHeight - 1,
			ValidActionHashes: []string{
				keeperCanonicalActionHash(500000),
			},
		},
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
