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

	var voteExtList []*types.VoteExt

	voteExtensions := q.k.GetAllVoteExt(ctx)

	for _, voteExt := range voteExtensions {
		if voteExt.Height != req.Height {
			continue
		}
		voteExtList = append(voteExtList, &voteExt)
	}

	return &types.QueryVoteextsByHeightResponse{VoteExts: voteExtList}, nil
}
