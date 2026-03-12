package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) GetValidatorCosmosPubKey(ctx context.Context, req *types.QueryGetValidatorCosmosPubKeyRequest) (*types.QueryGetValidatorCosmosPubKeyResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Check if the mina key exists in the MinaToCosmos map.
	exists, err := q.k.validatorMinaToCosmos.Has(sdkCtx, req.ValidatorMinaPubKey)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	if !exists {
		return nil, status.Error(codes.NotFound, "cosmos key not found for given mina key")
	}

	cosmosKey, err := q.k.validatorMinaToCosmos.Get(sdkCtx, req.ValidatorMinaPubKey)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryGetValidatorCosmosPubKeyResponse{
		ValidatorCosmosPubKey: cosmosKey,
	}, nil
}
