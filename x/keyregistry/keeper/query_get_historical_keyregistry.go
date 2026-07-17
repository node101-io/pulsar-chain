package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) GetHistoricalKeyregistry(ctx context.Context, req *types.QueryGetHistoricalKeyregistryRequest) (*types.QueryGetHistoricalKeyregistryResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	var registeredValidators []*types.RegisteredValidatorSetEntry

	for i, validator := range req.Validators {
		if validator == nil {
			return nil, status.Errorf(codes.InvalidArgument, "validator entry at index %d is nil", i)
		}

		if err := types.ValidateValidatorCosmosPublicKey(validator.ValidatorCosmosPubKey); err != nil {
			return nil, status.Errorf(codes.InvalidArgument,
				"invalid validator cosmos public key at index %d: %v", i, err)
		}

		exists, err := q.k.ValidatorCosmosToMinaHas(sdkCtx, validator.ValidatorCosmosPubKey)
		if err != nil {
			return nil, status.Errorf(codes.Internal,
				"failed to check validator registration at index %d", i)
		}
		if !exists {
			return nil, status.Errorf(codes.NotFound,
				"validator mina public key not found for cosmos public key at index %d", i)
		}

		minaPubKey, err := q.k.ValidatorGetCosmosToMina(sdkCtx, validator.ValidatorCosmosPubKey)
		if err != nil {
			return nil, status.Errorf(codes.Internal,
				"failed to get validator mina public key at index %d", i)
		}

		registeredValidators = append(registeredValidators, &types.RegisteredValidatorSetEntry{
			ValidatorCosmosPubKey: validator.ValidatorCosmosPubKey,
			ValidatorMinaPubKey:   minaPubKey,
			ConsensusPower:        validator.ConsensusPower,
		})
	}

	return &types.QueryGetHistoricalKeyregistryResponse{
		RegisteredValidators: registeredValidators,
	}, nil
}
