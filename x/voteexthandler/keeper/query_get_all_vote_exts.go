package keeper

import (
	"context"

	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) GetAllVoteExts(ctx context.Context, req *types.QueryGetAllVoteExtsRequest) (*types.QueryGetAllVoteExtsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	allVoteExts, err := q.k.GetAllVoteExt(ctx)
	if err != nil {
		return nil, err
	}

	return &types.QueryGetAllVoteExtsResponse{VoteExts: allVoteExts}, nil
}
