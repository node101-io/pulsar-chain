package keeper

import (
	"context"
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	minaField "github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ActionsReducedRoot returns the actions reduced root visible in the current
// SDK query context.
//
// Historical queries must use the standard Cosmos SDK block-height metadata:
//
//	x-cosmos-block-height: <height>
//
// If the node still retains the requested application state version, the
// response returns the root committed at that height. If the version has been
// pruned or no root exists in that version, the query must return NotFound and
// must not fall back to the latest available root.
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

	fieldElement, err := minaField.NewFieldElement(actionsRoot)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid actions reduced root")
	}

	return &types.QueryActionsReducedRootResponse{
		ActionsReducedRoot: fieldElement.String(),
	}, nil
}
