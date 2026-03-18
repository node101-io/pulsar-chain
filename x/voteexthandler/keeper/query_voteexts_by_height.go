package keeper

import (
	"context"

	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) VoteextsByHeight(ctx context.Context, req *types.QueryVoteextsByHeightRequest) (*types.QueryVoteextsByHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	voteExtensions := q.k.GetVoteExtsByHeight(ctx, req.Height)

	return &types.QueryVoteextsByHeightResponse{VoteExts: voteExtensions}, nil
}
