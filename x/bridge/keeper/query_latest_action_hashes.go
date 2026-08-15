package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) LatestActionHashes(ctx context.Context, req *types.QueryLatestActionHashesRequest) (*types.QueryLatestActionHashesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	bridgeState, err := q.k.GetBridgeState(sdkCtx)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "bridge state not found")
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	if err := bridgeState.Validate(); err != nil {
		return nil, status.Error(codes.Internal, "invalid bridge state")
	}

	return &types.QueryLatestActionHashesResponse{
		LatestFetchedMinaHeight:       bridgeState.LatestFetchedMinaHeight,
		ActionHashes:                  bridgeState.ActionHashes,
		StartMinaHeight:               bridgeState.StartMinaHeight,
		ActionHashesCosmosBlockHeight: bridgeState.ActionHashesCosmosBlockHeight,
	}, nil
}
