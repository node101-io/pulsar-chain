package keeper_test

import (
	"testing"

	"github.com/node101-io/pulsar-chain/x/votepersistence/keeper"
	"github.com/node101-io/pulsar-chain/x/votepersistence/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestVoteExtensionsInvalidArgumentFail(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err := qs.VoteExtensions(f.ctx, nil)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestVoteExtensionsEmptyStore(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx.WithBlockHeight(17)

	qs := keeper.NewQueryServerImpl(f.keeper)

	response, err := qs.VoteExtensions(ctx, &types.QueryVoteExtensionsRequest{})
	require.NoError(t, err)
	require.Equal(t, int64(17), response.QueryBlockHeight)
	require.Zero(t, response.PersistedVoteExtensionsBlockHeight)
	require.Empty(t, response.VoteExtensions)
}

func TestVoteExtensionsSuccess(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx.WithBlockHeight(50)

	entries := []voteEntry{
		{blockHeight: 42, minaAddress: []byte("mina-b"), voteExtensions: []byte("vote-b")},
		{blockHeight: 42, minaAddress: []byte("mina-a"), voteExtensions: []byte("vote-a")},
	}

	for _, entry := range entries {
		require.NoError(t, f.keeper.SetVote(f.ctx, entry.blockHeight, entry.minaAddress, entry.voteExtensions))
	}

	qs := keeper.NewQueryServerImpl(f.keeper)

	response, err := qs.VoteExtensions(ctx, &types.QueryVoteExtensionsRequest{})
	require.NoError(t, err)
	require.Equal(t, int64(50), response.QueryBlockHeight)
	require.Equal(t, int64(42), response.PersistedVoteExtensionsBlockHeight)

	expected := []*types.StoredVoteExtension{
		{MinaPublicKey: []byte("mina-a"), VoteExtension: []byte("vote-a")},
		{MinaPublicKey: []byte("mina-b"), VoteExtension: []byte("vote-b")},
	}
	require.Equal(t, expected, response.VoteExtensions)
}

func TestVoteExtensionsMultiplePersistedHeightsFail(t *testing.T) {
	f := initFixture(t)

	require.NoError(t, f.keeper.SetVote(f.ctx, 42, []byte("mina-a"), []byte("vote-a")))
	require.NoError(t, f.keeper.SetVote(f.ctx, 43, []byte("mina-b"), []byte("vote-b")))

	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err := qs.VoteExtensions(f.ctx, &types.QueryVoteExtensionsRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Internal, st.Code())
}
