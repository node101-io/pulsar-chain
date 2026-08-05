package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/node101-io/pulsar-chain/x/bridge/keeper"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

func TestLatestValidActionHashesInvalidArgumentFail(t *testing.T) {
	f := initFixture(t, nil, nil, nil)

	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err := qs.LatestValidActionHashes(f.ctx, nil)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestLatestValidActionHashesNotFound(t *testing.T) {
	f := initFixture(t, nil, nil, nil)

	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err := qs.LatestValidActionHashes(f.ctx, &types.QueryLatestValidActionHashesRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.NotFound, st.Code())
}

func TestLatestValidActionHashesSuccessWithEmptyHashes(t *testing.T) {
	f := initFixture(t, nil, nil, nil)

	require.NoError(t, f.keeper.BridgeState.Set(f.ctx, types.BridgeState{
		LatestFetchedMinaHeight: 0,
		ValidActionHashes:       nil,
	}))

	qs := keeper.NewQueryServerImpl(f.keeper)

	response, err := qs.LatestValidActionHashes(f.ctx, &types.QueryLatestValidActionHashesRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryLatestValidActionHashesResponse{
		LatestFetchedMinaHeight: 0,
		ValidActionHashes:       nil,
	}, response)
}
