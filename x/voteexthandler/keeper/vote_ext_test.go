package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

// TestSetAndGetVoteExt verifies that SetVoteExt stores the VoteExt
// and GetVoteExt retrieves it correctly.
func TestSetAndGetVoteExt(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx
	k := f.keeper

	vote := types.VoteExt{
		Index:         "vote1",
		Height:        100,
		ValidatorAddr: "validator1",
		Signature:     []byte("sig1"),
	}

	err := k.SetVoteExt(ctx, vote)
	require.NoError(t, err)

	found, err := k.HasVoteExt(ctx, "vote1")
	require.NoError(t, err)
	require.True(t, found)

	got, err := k.GetVoteExt(ctx, "vote1")
	require.NoError(t, err)
	require.Equal(t, vote, got)

	found, err = k.HasVoteExt(ctx, "nonexistent")
	require.NoError(t, err)
	require.False(t, found)
}

// TestRemoveVoteExt verifies that RemoveVoteExt deletes the VoteExt.
func TestRemoveVoteExt(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx
	k := f.keeper

	vote := types.VoteExt{
		Index:  "vote2",
		Height: 101,
	}

	_ = k.SetVoteExt(ctx, vote)

	err := k.RemoveVoteExt(ctx, "vote2")
	require.NoError(t, err)

	found, err := k.HasVoteExt(ctx, "vote2")
	require.NoError(t, err)
	require.False(t, found)
}

// TestGetAllVoteExt verifies that GetAllVoteExt returns all stored VoteExts.
func TestGetAllVoteExt(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx
	k := f.keeper

	votes := []types.VoteExt{
		{Index: "v1", Height: 100},
		{Index: "v2", Height: 101},
		{Index: "v3", Height: 100},
	}

	for _, v := range votes {
		_ = k.SetVoteExt(ctx, v)
	}

	all, err := k.GetAllVoteExt(ctx)
	require.NoError(t, err)
	require.Len(t, all, 3)

	for _, v := range votes {
		found := false
		for _, a := range all {
			if a.Index == v.Index {
				found = true
				break
			}
		}
		require.True(t, found)
	}
}

// TestGetVoteExtsByHeight verifies that GetVoteExtsByHeight returns
// only the VoteExts matching the given height.
func TestGetVoteExtsByHeight(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx
	k := f.keeper

	votes := []types.VoteExt{
		{Index: "v1", Height: 100},
		{Index: "v2", Height: 101},
		{Index: "v3", Height: 100},
	}

	for _, v := range votes {
		_ = k.SetVoteExt(ctx, v)
	}

	h100 := k.GetVoteExtsByHeight(ctx, 100)
	require.Len(t, h100, 2)

	h101 := k.GetVoteExtsByHeight(ctx, 101)
	require.Len(t, h101, 1)

	h999 := k.GetVoteExtsByHeight(ctx, 999)
	require.Len(t, h999, 0)
}

// TestRemoveAllVoteExts verifies that RemoveAllVoteExts deletes all stored VoteExts.
func TestRemoveAllVoteExts(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx
	k := f.keeper

	votes := []types.VoteExt{
		{Index: "v1", Height: 100},
		{Index: "v2", Height: 101},
		{Index: "v3", Height: 102},
	}

	for _, v := range votes {
		err := k.SetVoteExt(ctx, v)
		require.NoError(t, err)

		existence, err := k.HasVoteExt(ctx, v.Index)
		require.NoError(t, err)
		require.True(t, existence)
	}

	err := k.RemoveAllVoteExts(ctx)
	require.NoError(t, err)

	all, err := k.GetAllVoteExt(ctx)
	require.NoError(t, err)
	require.Len(t, all, 0)

	for _, v := range votes {
		found, err := k.HasVoteExt(ctx, v.Index)
		require.NoError(t, err)
		require.False(t, found)
	}
}
