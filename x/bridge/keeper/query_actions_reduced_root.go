package keeper

import (
	"context"
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) ActionsReducedRoot(ctx context.Context, req *types.QueryActionsReducedRootRequest) (*types.QueryActionsReducedRootResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	actionsRoot, err := q.k.GetLatestActionsReducedRoot(sdkCtx)
	if err != nil {
		if errors.Is(err, types.ErrActionsReducedRootSnapshotNotFound) {
			return nil, status.Error(codes.NotFound, "actions reduced root not found")
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	return &types.QueryActionsReducedRootResponse{
		ActionsReducedRoot: actionsRoot,
	}, nil
}
