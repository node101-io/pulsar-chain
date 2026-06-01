package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetMinaPubKey returns the validator's mina public key associated with the given cosmos public key.
// Returns NotFound if no mapping exists for the provided cosmos public key.
func (q queryServer) GetValidatorMinaPubKey(ctx context.Context, req *types.QueryGetValidatorMinaPubKeyRequest) (*types.QueryGetValidatorMinaPubKeyResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if err := types.ValidateValidatorCosmosPublicKey(req.ValidatorCosmosPubKey); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Check if the cosmos key exists in the CosmosToMina map.
	exists, err := q.k.validatorCosmosToMina.Has(sdkCtx, req.ValidatorCosmosPubKey)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal Error")
	}

	if !exists {
		return nil, status.Error(codes.NotFound, "mina key not found for given cosmos key")
	}

	minaKey, err := q.k.validatorCosmosToMina.Get(sdkCtx, req.ValidatorCosmosPubKey)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryGetValidatorMinaPubKeyResponse{
		ValidatorMinaPubKey: minaKey,
	}, nil
}
