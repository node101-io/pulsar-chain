package keeper_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/node101-io/pulsar-chain/x/smartaccounts/keeper"
	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestGetSessionKeysByIdentityPagination verifies default page-size enforcement
// and continuation through the returned pagination key.
func TestGetSessionKeysByIdentityPagination(t *testing.T) {
	f := initFixture(t)
	identity := bytes.Repeat([]byte{0x01}, types.IdentitySize)
	accountAddress := f.keeper.GetAuthority()
	expiresAtHeight := uint64(sdk.UnwrapSDKContext(f.ctx).BlockHeight()) + 1_000

	for i := uint64(1); i <= 105; i++ {
		publicKey := make([]byte, types.SessionPublicKeySize)
		binary.BigEndian.PutUint64(publicKey[types.SessionPublicKeySize-8:], i)
		require.NoError(t, f.keeper.AppendSessionKeyToSmartAccount(
			f.ctx,
			identity,
			accountAddress,
			types.SessionKey{PublicKey: publicKey, ExpiresAtHeight: expiresAtHeight},
		))
	}

	server := keeper.NewQueryServerImpl(f.keeper)
	firstPage, err := server.GetSessionKeysByIdentity(
		f.ctx,
		&types.QueryGetSessionKeysByIdentityRequest{Identity: identity},
	)
	require.NoError(t, err)
	require.Len(t, firstPage.SessionKeys, 100)
	require.Equal(t, uint64(105), firstPage.Pagination.Total)
	require.Len(t, firstPage.Pagination.NextKey, 8)

	secondPage, err := server.GetSessionKeysByIdentity(
		f.ctx,
		&types.QueryGetSessionKeysByIdentityRequest{
			Identity: identity,
			Pagination: &query.PageRequest{
				Key:   firstPage.Pagination.NextKey,
				Limit: 100,
			},
		},
	)
	require.NoError(t, err)
	require.Len(t, secondPage.SessionKeys, 5)
	require.Equal(t, uint64(105), secondPage.Pagination.Total)
	require.Empty(t, secondPage.Pagination.NextKey)
}

// TestGetSessionKeysByIdentityRejectsInvalidPagination verifies that unsupported
// or malformed pagination options return InvalidArgument.
func TestGetSessionKeysByIdentityRejectsInvalidPagination(t *testing.T) {
	f := initFixture(t)
	identity := bytes.Repeat([]byte{0x01}, types.IdentitySize)
	require.NoError(t, f.keeper.AppendSessionKeyToSmartAccount(
		f.ctx,
		identity,
		f.keeper.GetAuthority(),
		types.SessionKey{
			PublicKey:       bytes.Repeat([]byte{0x02}, types.SessionPublicKeySize),
			ExpiresAtHeight: uint64(sdk.UnwrapSDKContext(f.ctx).BlockHeight()) + 100,
		},
	))

	server := keeper.NewQueryServerImpl(f.keeper)
	tests := []struct {
		name       string
		pagination *query.PageRequest
	}{
		{name: "reverse", pagination: &query.PageRequest{Reverse: true}},
		{name: "malformed key", pagination: &query.PageRequest{Key: []byte{0x01}}},
		{name: "key and offset", pagination: &query.PageRequest{Key: make([]byte, 8), Offset: 1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := server.GetSessionKeysByIdentity(
				f.ctx,
				&types.QueryGetSessionKeysByIdentityRequest{
					Identity:   identity,
					Pagination: test.pagination,
				},
			)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

// TestGetSessionKeysByIdentityOffsetAndLimit verifies offset-based pagination,
// session-key ordering, account ownership, total count, and the next-page key.
func TestGetSessionKeysByIdentityOffsetAndLimit(t *testing.T) {
	f := initFixture(t)
	identity := bytes.Repeat([]byte{0x01}, types.IdentitySize)
	accountAddress := f.keeper.GetAuthority()
	expiresAtHeight := uint64(sdk.UnwrapSDKContext(f.ctx).BlockHeight()) + 100

	for i := byte(1); i <= 5; i++ {
		publicKey := bytes.Repeat([]byte{i}, types.SessionPublicKeySize)
		require.NoError(t, f.keeper.AppendSessionKeyToSmartAccount(
			f.ctx,
			identity,
			accountAddress,
			types.SessionKey{PublicKey: publicKey, ExpiresAtHeight: expiresAtHeight},
		))
	}

	response, err := keeper.NewQueryServerImpl(f.keeper).GetSessionKeysByIdentity(
		f.ctx,
		&types.QueryGetSessionKeysByIdentityRequest{
			Identity: identity,
			Pagination: &query.PageRequest{
				Offset: 2,
				Limit:  2,
			},
		},
	)
	require.NoError(t, err)
	require.Equal(t, accountAddress, response.AccountAddress)
	require.Len(t, response.SessionKeys, 2)
	require.Equal(t, bytes.Repeat([]byte{0x03}, types.SessionPublicKeySize), response.SessionKeys[0].PublicKey)
	require.Equal(t, bytes.Repeat([]byte{0x04}, types.SessionPublicKeySize), response.SessionKeys[1].PublicKey)
	require.Equal(t, uint64(5), response.Pagination.Total)
	require.Equal(t, uint64(4), binary.BigEndian.Uint64(response.Pagination.NextKey))
}

// TestGetSessionKeysByIdentityRejectsInvalidRequest verifies request and identity
// validation, including the response for an unknown identity.
func TestGetSessionKeysByIdentityRejectsInvalidRequest(t *testing.T) {
	f := initFixture(t)
	server := keeper.NewQueryServerImpl(f.keeper)

	tests := []struct {
		name string
		req  *types.QueryGetSessionKeysByIdentityRequest
		want codes.Code
	}{
		{name: "nil request", want: codes.InvalidArgument},
		{
			name: "empty identity",
			req:  &types.QueryGetSessionKeysByIdentityRequest{},
			want: codes.InvalidArgument,
		},
		{
			name: "invalid identity size",
			req:  &types.QueryGetSessionKeysByIdentityRequest{Identity: []byte{0x01}},
			want: codes.InvalidArgument,
		},
		{
			name: "unknown identity",
			req: &types.QueryGetSessionKeysByIdentityRequest{
				Identity: bytes.Repeat([]byte{0x01}, types.IdentitySize),
			},
			want: codes.NotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := server.GetSessionKeysByIdentity(f.ctx, test.req)
			require.Equal(t, test.want, status.Code(err))
		})
	}
}
