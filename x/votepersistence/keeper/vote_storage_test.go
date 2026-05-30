package keeper_test

import (
	"bytes"
	"testing"

	"cosmossdk.io/collections"
	"github.com/stretchr/testify/require"
)

type voteEntry struct {
	blockHeight    int64
	minaAddress    []byte
	voteExtensions []byte
}

func TestVoteStorageSetGetHas(t *testing.T) {
	f := initFixture(t)

	blockHeight := int64(42)
	minaAddress := []byte("mina-address-1")
	voteExtensions := []byte("vote-extension-1")

	exists, err := f.keeper.VoteExists(f.ctx, blockHeight, minaAddress)
	require.NoError(t, err)
	require.False(t, exists)

	err = f.keeper.SetVote(f.ctx, blockHeight, minaAddress, voteExtensions)
	require.NoError(t, err)

	got, err := f.keeper.GetVote(f.ctx, blockHeight, minaAddress)
	require.NoError(t, err)
	require.Equal(t, voteExtensions, got)

	exists, err = f.keeper.VoteExists(f.ctx, blockHeight, minaAddress)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestVoteStorageGetFailsWhenVoteMissing(t *testing.T) {
	f := initFixture(t)

	_, err := f.keeper.GetVote(f.ctx, 42, []byte("missing-mina-address"))
	require.ErrorIs(t, err, collections.ErrNotFound)
}

func TestVoteStorageRemove(t *testing.T) {
	f := initFixture(t)

	blockHeight := int64(42)
	minaAddress := []byte("mina-address-1")
	voteExtensions := []byte("vote-extension-1")

	err := f.keeper.SetVote(f.ctx, blockHeight, minaAddress, voteExtensions)
	require.NoError(t, err)

	exists, err := f.keeper.VoteExists(f.ctx, blockHeight, minaAddress)
	require.NoError(t, err)
	require.True(t, exists)

	err = f.keeper.RemoveVote(f.ctx, blockHeight, minaAddress)
	require.NoError(t, err)

	exists, err = f.keeper.VoteExists(f.ctx, blockHeight, minaAddress)
	require.NoError(t, err)
	require.False(t, exists)

	_, err = f.keeper.GetVote(f.ctx, blockHeight, minaAddress)
	require.ErrorIs(t, err, collections.ErrNotFound)
}

func TestVoteStorageRemoveVotes(t *testing.T) {
	f := initFixture(t)

	entries := []voteEntry{
		{blockHeight: 3, minaAddress: []byte("mina-a"), voteExtensions: []byte("vote-a")},
		{blockHeight: 3, minaAddress: []byte("mina-b"), voteExtensions: []byte("vote-b")},
		{blockHeight: 8, minaAddress: []byte("mina-c"), voteExtensions: []byte("vote-c")},
	}

	for _, entry := range entries {
		err := f.keeper.SetVote(f.ctx, entry.blockHeight, entry.minaAddress, entry.voteExtensions)
		require.NoError(t, err)
	}

	err := f.keeper.Clear(f.ctx)
	require.NoError(t, err)

	for _, entry := range entries {
		exists, err := f.keeper.VoteExists(f.ctx, entry.blockHeight, entry.minaAddress)
		require.NoError(t, err)
		require.False(t, exists)
	}

	iter, err := f.keeper.IterateVotes(f.ctx)
	require.NoError(t, err)
	defer iter.Close()

	require.False(t, iter.Valid())
}

func TestVoteStorageIterate(t *testing.T) {
	f := initFixture(t)

	entries := []voteEntry{
		{blockHeight: 8, minaAddress: []byte("mina-b"), voteExtensions: []byte("vote-b")},
		{blockHeight: 3, minaAddress: []byte("mina-c"), voteExtensions: []byte("vote-c")},
		{blockHeight: 3, minaAddress: []byte("mina-a"), voteExtensions: []byte("vote-a")},
	}

	for _, entry := range entries {
		err := f.keeper.SetVote(f.ctx, entry.blockHeight, entry.minaAddress, entry.voteExtensions)
		require.NoError(t, err)
	}

	iter, err := f.keeper.IterateVotes(f.ctx)
	require.NoError(t, err)
	defer iter.Close()

	var got []voteEntry
	for iter.Valid() {
		key, err := iter.Key()
		require.NoError(t, err)

		value, err := iter.Value()
		require.NoError(t, err)

		got = append(got, voteEntry{
			blockHeight:    key.K1(),
			minaAddress:    bytes.Clone(key.K2()),
			voteExtensions: bytes.Clone(value),
		})

		iter.Next()
	}

	expected := []voteEntry{
		{blockHeight: 3, minaAddress: []byte("mina-a"), voteExtensions: []byte("vote-a")},
		{blockHeight: 3, minaAddress: []byte("mina-c"), voteExtensions: []byte("vote-c")},
		{blockHeight: 8, minaAddress: []byte("mina-b"), voteExtensions: []byte("vote-b")},
	}

	require.Equal(t, expected, got)
}
