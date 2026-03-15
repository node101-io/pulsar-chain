package keeper_test

import (
	"testing"

	"github.com/node101-io/pulsar-chain/x/voteexthandler/keeper"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
	"github.com/stretchr/testify/require"
)

// TestVoteextsByHeight_ValidHeight verifies that querying a valid height returns correct VoteExts
func TestVoteextsByHeight_ValidHeight(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx
	q := keeper.NewQueryServerImpl(f.keeper)

	// Setup votes
	v1 := types.VoteExt{Index: "v1", Height: 100, ValidatorAddr: "val1", Signature: "sig1"}
	v2 := types.VoteExt{Index: "v2", Height: 100, ValidatorAddr: "val2", Signature: "sig2"}
	_ = f.keeper.SetVoteExt(ctx, v1)
	_ = f.keeper.SetVoteExt(ctx, v2)

	req := &types.QueryVoteextsByHeightRequest{Height: 100}
	resp, err := q.VoteextsByHeight(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.VoteExts, 2)
}

// TestVoteextsByHeight_NonexistentHeight verifies that querying a height with no votes returns empty
func TestVoteextsByHeight_NonexistentHeight(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx
	q := keeper.NewQueryServerImpl(f.keeper)

	req := &types.QueryVoteextsByHeightRequest{Height: 999}
	resp, err := q.VoteextsByHeight(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.VoteExts, 0)
}

// TestVoteextsByHeight_MultipleVotesDifferentHeights verifies that only votes matching the height are returned
func TestVoteextsByHeight_MultipleVotesDifferentHeights(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx
	q := keeper.NewQueryServerImpl(f.keeper)

	votes := []types.VoteExt{
		{Index: "v1", Height: 100},
		{Index: "v2", Height: 101},
		{Index: "v3", Height: 100},
		{Index: "v4", Height: 102},
	}
	for _, v := range votes {
		_ = f.keeper.SetVoteExt(ctx, v)
	}

	req := &types.QueryVoteextsByHeightRequest{Height: 100}
	resp, err := q.VoteextsByHeight(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.VoteExts, 2)

	// Verify indexes returned
	found := map[string]bool{}
	for _, v := range resp.VoteExts {
		found[v.Index] = true
	}
	require.True(t, found["v1"])
	require.True(t, found["v3"])
}

// TestVoteextsByHeight_InvalidRequest verifies that nil request returns gRPC error
func TestVoteextsByHeight_InvalidRequest(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx
	q := keeper.NewQueryServerImpl(f.keeper)

	resp, err := q.VoteextsByHeight(ctx, nil)
	require.Nil(t, resp)
	require.Error(t, err)
}
