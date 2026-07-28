package keeper_test

import (
	"testing"

	"github.com/node101-io/pulsar-chain/x/bridge/types"
	"github.com/stretchr/testify/require"
)

func TestGetActionsReducedRootAtHeightReturnsLatestSnapshotAtOrBeforeHeight(t *testing.T) {
	f := initFixture(t)

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

func TestGetLatestActionsReducedRootReturnsHighestSnapshot(t *testing.T) {
	f := initFixture(t)

	require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(f.ctx, 0, []byte("root-0")))
	require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(f.ctx, 5, []byte("root-5")))

	root, err := f.keeper.GetLatestActionsReducedRoot(f.ctx)
	require.NoError(t, err)
	require.Equal(t, []byte("root-5"), root)
}

func TestGetActionsReducedRootAtHeightRejectsNegativeHeight(t *testing.T) {
	f := initFixture(t)

	_, err := f.keeper.GetActionsReducedRootAtHeight(f.ctx, -1)
	require.ErrorIs(t, err, types.ErrInvalidBridgeStateHeight)
}
