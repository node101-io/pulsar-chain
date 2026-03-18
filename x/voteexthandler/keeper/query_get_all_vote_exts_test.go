package keeper_test

import (
	"testing"

	"github.com/node101-io/pulsar-chain/x/voteexthandler/keeper"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
	"github.com/stretchr/testify/require"
)

// TestGetAllVoteExts_Empty verifies that querying with no votes returns empty slice
func TestGetAllVoteExts_Empty(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx
	q := keeper.NewQueryServerImpl(f.keeper)

	resp, err := q.GetAllVoteExts(ctx, &types.QueryGetAllVoteExtsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.VoteExts, 0)
}

// TestGetAllVoteExts_SingleVote verifies that a single stored vote is returned
func TestGetAllVoteExts_SingleVote(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx
	q := keeper.NewQueryServerImpl(f.keeper)

	v := types.VoteExt{Index: "v1", Height: 100, ValidatorAddr: "val1", Signature: "sig1"}
	err := f.keeper.SetVoteExt(ctx, v)
	require.NoError(t, err)

	resp, err := q.GetAllVoteExts(ctx, &types.QueryGetAllVoteExtsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.VoteExts, 1)
	require.Equal(t, "v1", resp.VoteExts[0].Index)
}

// TestGetAllVoteExts_MultipleVotes verifies that all stored votes across heights are returned
func TestGetAllVoteExts_MultipleVotes(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx
	q := keeper.NewQueryServerImpl(f.keeper)

	votes := []types.VoteExt{
		{Index: "v1", Height: 100, ValidatorAddr: "val1", Signature: "sig1"},
		{Index: "v2", Height: 101, ValidatorAddr: "val2", Signature: "sig2"},
		{Index: "v3", Height: 102, ValidatorAddr: "val3", Signature: "sig3"},
	}
	for _, v := range votes {
		err := f.keeper.SetVoteExt(ctx, v)
		require.NoError(t, err)
	}

	resp, err := q.GetAllVoteExts(ctx, &types.QueryGetAllVoteExtsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.VoteExts, 3)

	found := map[string]bool{}
	for _, v := range resp.VoteExts {
		found[v.Index] = true
	}
	require.True(t, found["v1"])
	require.True(t, found["v2"])
	require.True(t, found["v3"])
}

// TestGetAllVoteExts_InvalidRequest verifies that nil request returns gRPC error
func TestGetAllVoteExts_InvalidRequest(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx
	q := keeper.NewQueryServerImpl(f.keeper)

	resp, err := q.GetAllVoteExts(ctx, nil)
	require.Nil(t, resp)
	require.Error(t, err)
}
