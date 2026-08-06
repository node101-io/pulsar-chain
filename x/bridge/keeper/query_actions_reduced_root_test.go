package keeper_test

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minafield "github.com/node101-io/mina-signer-go/field"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/node101-io/pulsar-chain/x/bridge/keeper"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

func queryCanonicalActionsReducedRootString(v uint64) string {
	return minafield.NewField().FromUint64(v).String()
}

func TestActionsReducedRootInvalidArgumentFail(t *testing.T) {
	f := initFixture(t, nil, nil, nil)

	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err := qs.ActionsReducedRoot(f.ctx, nil)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestActionsReducedRootNotFound(t *testing.T) {
	f := initFixture(t, nil, nil, nil)
	sdkCtx := sdk.UnwrapSDKContext(f.ctx)
	cms, ok := sdkCtx.MultiStore().(storetypes.CommitMultiStore)
	require.True(t, ok)

	for height := int64(1); height <= 8; height++ {
		sdkCtx = sdkCtx.WithBlockHeight(height)

		switch height {
		case 3:
			require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(sdkCtx, 3, keeperCanonicalRoot(3)))
		case 8:
			require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(sdkCtx, 8, keeperCanonicalRoot(8)))
		}

		cms.Commit()
	}

	historicalStore, err := cms.CacheMultiStoreWithVersion(2)
	require.NoError(t, err)

	header := sdkCtx.BlockHeader()
	header.Height = 2
	historicalCtx := sdk.NewContext(historicalStore, header, true, sdkCtx.Logger())

	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err = qs.ActionsReducedRoot(historicalCtx, &types.QueryActionsReducedRootRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.NotFound, st.Code())
}

func TestActionsReducedRootSuccess(t *testing.T) {
	f := initFixture(t, nil, nil, nil)
	ctx := sdk.UnwrapSDKContext(f.ctx).WithBlockHeight(8)

	require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(ctx, 3, keeperCanonicalRoot(3)))
	require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(ctx, 8, keeperCanonicalRoot(8)))

	qs := keeper.NewQueryServerImpl(f.keeper)

	response, err := qs.ActionsReducedRoot(ctx, &types.QueryActionsReducedRootRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryActionsReducedRootResponse{
		ActionsReducedRoot: queryCanonicalActionsReducedRootString(8),
	}, response)
}

func TestActionsReducedRootInvalidStoredRoot(t *testing.T) {
	f := initFixture(t, nil, nil, nil)
	ctx := sdk.UnwrapSDKContext(f.ctx).WithBlockHeight(8)

	require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(ctx, 8, []byte{1, 2, 3}))

	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err := qs.ActionsReducedRoot(ctx, &types.QueryActionsReducedRootRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Internal, st.Code())
}

func TestActionsReducedRootHistoricalQueryReturnsHistoricalState(t *testing.T) {
	f := initFixture(t, nil, nil, nil)

	qs := keeper.NewQueryServerImpl(f.keeper)
	sdkCtx := sdk.UnwrapSDKContext(f.ctx)
	cms, ok := sdkCtx.MultiStore().(storetypes.CommitMultiStore)
	require.True(t, ok)

	rootsByHeight := map[int64][]byte{
		3: keeperCanonicalRoot(3),
		4: keeperCanonicalRoot(4),
		5: keeperCanonicalRoot(5),
	}

	for height := int64(1); height <= 5; height++ {
		sdkCtx = sdkCtx.WithBlockHeight(height)

		if root, exists := rootsByHeight[height]; exists {
			require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(sdkCtx, height, root))
		}

		cms.Commit()
	}

	historicalStore, err := cms.CacheMultiStoreWithVersion(3)
	require.NoError(t, err)

	header := sdkCtx.BlockHeader()
	header.Height = 3
	historicalCtx := sdk.NewContext(historicalStore, header, true, sdkCtx.Logger())

	response, err := qs.ActionsReducedRoot(historicalCtx, &types.QueryActionsReducedRootRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryActionsReducedRootResponse{
		ActionsReducedRoot: queryCanonicalActionsReducedRootString(3),
	}, response)
}
