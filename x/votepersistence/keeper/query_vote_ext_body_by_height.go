package keeper

import (
	"context"

	"github.com/node101-io/pulsar-chain/x/votepersistence/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) VoteExtBodyByHeight(ctx context.Context, req *types.QueryVoteExtBodyByHeightRequest) (*types.QueryVoteExtBodyByHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	return &types.QueryVoteExtBodyByHeightResponse{}, nil
}
