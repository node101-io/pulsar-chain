package keeper

import (
	"bytes"
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/votepersistence/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) VoteExtensions(ctx context.Context, req *types.QueryVoteExtensionsRequest) (*types.QueryVoteExtensionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	response := &types.QueryVoteExtensionsResponse{
		QueryBlockHeight: sdkCtx.BlockHeight(),
	}

	iter, err := q.k.IterateVotes(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to iterate persisted vote extensions")
	}
	defer iter.Close()

	var (
		persistedBlockHeight int64
		foundVotes           bool
	)

	for ; iter.Valid(); iter.Next() {
		key, err := iter.Key()
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to read persisted vote extension key")
		}

		voteExtension, err := iter.Value()
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to read persisted vote extension")
		}

		blockHeight := key.K1()
		if !foundVotes {
			persistedBlockHeight = blockHeight
			response.PersistedVoteExtensionsBlockHeight = blockHeight
			foundVotes = true
		} else if blockHeight != persistedBlockHeight {
			return nil, status.Error(codes.Internal, "persisted vote extensions contain multiple block heights")
		}

		response.VoteExtensions = append(response.VoteExtensions, &types.StoredVoteExtension{
			MinaPublicKey: bytes.Clone(key.K2()),
			VoteExtension: bytes.Clone(voteExtension),
		})
	}

	return response, nil
}
