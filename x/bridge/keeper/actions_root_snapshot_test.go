package keeper_test

import (
	"testing"

	"github.com/node101-io/pulsar-chain/x/bridge/types"
	"github.com/stretchr/testify/require"
)

func TestGetActionsReducedRootAtHeightReturnsLatestSnapshotAtOrBeforeHeight(t *testing.T) {
	f := initFixture(t, nil, nil, nil)

	require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(f.ctx, 0, []byte("root-0")))
	require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(f.ctx, 5, []byte("root-5")))
	require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(f.ctx, 9, []byte("root-9")))

	root, err := f.keeper.GetActionsReducedRootAtHeight(f.ctx, 7)
	require.NoError(t, err)
	require.Equal(t, []byte("root-5"), root)

	root, err = f.keeper.GetActionsReducedRootAtHeight(f.ctx, 9)
	require.NoError(t, err)
	require.Equal(t, []byte("root-9"), root)
}

func TestGetLatestActionsReducedRootReturnsNewestSnapshot(t *testing.T) {
	f := initFixture(t, nil, nil, nil)

	require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(f.ctx, 0, []byte("root-0")))
	require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(f.ctx, 5, []byte("root-5")))

	root, err := f.keeper.GetLatestActionsReducedRoot(f.ctx)
	require.NoError(t, err)
	require.Equal(t, []byte("root-5"), root)
}

func TestGetActionsReducedRootAtHeightRejectsNegativeHeight(t *testing.T) {
	f := initFixture(t, nil, nil, nil)

	_, err := f.keeper.GetActionsReducedRootAtHeight(f.ctx, -1)
	require.ErrorIs(t, err, types.ErrInvalidBridgeStateHeight)
}

func TestSetActionsReducedRootPrunesToRollingWindow(t *testing.T) {
	f := initFixture(t, nil, nil, nil)
	require.NoError(t, f.keeper.BridgeState.Set(f.ctx, types.DefaultTestBridgeState()))
	windowSize := validBridgeParams().ActionsReducedRootSnapshotWindowSize

	for height := int64(1); height <= windowSize+1; height++ {
		require.NoError(t, f.keeper.SetActionsReducedRoot(f.ctx, height, []byte{byte(height)}))
	}

	iter, err := f.keeper.ActionsReducedRootSnapshots.Iterate(f.ctx, nil)
	require.NoError(t, err)
	defer iter.Close()

	var heights []int64
	for ; iter.Valid(); iter.Next() {
		height, err := iter.Key()
		require.NoError(t, err)
		heights = append(heights, height)
	}

	require.Equal(t, []int64{2, 3, 4, 5}, heights)

	_, err = f.keeper.GetActionsReducedRootAtHeight(f.ctx, 1)
	require.ErrorIs(t, err, types.ErrActionsReducedRootSnapshotNotFound)

	root, err := f.keeper.GetLatestActionsReducedRoot(f.ctx)
	require.NoError(t, err)
	require.Equal(t, []byte{5}, root)
}
