package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetCosmosPubKey returns the user's cosmos public key associated with the given mina public key.
// Returns NotFound if no mapping exists for the provided mina public key.
func (q queryServer) GetUserCosmosPublicKey(ctx context.Context, req *types.QueryGetUserCosmosPublicKeyRequest) (*types.QueryGetUserCosmosPublicKeyResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if err := types.ValidateMinaPublicKey(req.UserMinaPublicKey); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Check if the mina PublicKey exists in the MinaToCosmos map.
	exists, err := q.k.userMinaToCosmos.Has(sdkCtx, req.UserMinaPublicKey)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	if !exists {
		return nil, status.Error(codes.NotFound, "cosmos key not found for given mina key")
	}

	cosmosKey, err := q.k.userMinaToCosmos.Get(sdkCtx, req.UserMinaPublicKey)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryGetUserCosmosPublicKeyResponse{
		UserCosmosPublicKey: cosmosKey,
	}, nil
}
