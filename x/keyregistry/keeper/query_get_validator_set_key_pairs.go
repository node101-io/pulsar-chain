package keeper

import (
	"bytes"
	"context"
	"fmt"
	"sort"

	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) GetValidatorSetByHeight(
	ctx context.Context,
	req *types.QueryGetValidatorSetByHeightRequest,
) (*types.QueryGetValidatorSetByHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.BlockHeight <= 0 {
		return nil, fmt.Errorf("negative block height")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	if sdkCtx.BlockHeight() == req.BlockHeight {
		var validatorSet []*types.ValidatorSetEntry

		var iterErr error

		err := q.k.stakingKeeper.IterateLastValidators(ctx, func(index int64, validator stakingTypes.ValidatorI) bool {
			entry, err := q.newValidatorSetEntry(ctx, validator)
			if err != nil {
				iterErr = err
				return true
			}

			validatorSet = append(validatorSet, entry)
			return false
		})

		if err != nil {
			return nil, err
		}
		if iterErr != nil {
			return nil, iterErr
		}

		if err := sortValidatorsByPower(validatorSet); err != nil {
			return nil, err
		}

		return &types.QueryGetValidatorSetByHeightResponse{
			Validators: validatorSet,
		}, nil
	}

	historicalData, err := q.k.stakingKeeper.GetHistoricalInfo(ctx, req.BlockHeight)
	if err != nil {
		return nil, err
	}

	validatorSet := make([]*types.ValidatorSetEntry, 0, len(historicalData.Valset))
	for _, validator := range historicalData.Valset {
		entry, err := q.newValidatorSetEntry(ctx, validator)
		if err != nil {
			return nil, err
		}

		validatorSet = append(validatorSet, entry)
	}

	return &types.QueryGetValidatorSetByHeightResponse{
		Validators: validatorSet,
	}, nil
}

func sortValidatorsByPower(validators []*types.ValidatorSetEntry) error {

	sort.SliceStable(validators, func(i, j int) bool {
		if validators[i].ConsensusPower == validators[j].ConsensusPower {

			cmtPubKeyValidatorA := cmted25519.PubKey(validators[i].ValidatorCosmosPubKey)
			consAddrValidatorA := sdk.ConsAddress(cmtPubKeyValidatorA.Address())

			cmtPubKeyValidatorB := cmted25519.PubKey(validators[j].ValidatorCosmosPubKey)
			consAddrValidatorB := sdk.ConsAddress(cmtPubKeyValidatorB.Address())

			return bytes.Compare(consAddrValidatorA, consAddrValidatorB) < 0
		}

		return validators[i].ConsensusPower > validators[j].ConsensusPower
	})

	return nil
}

func (q queryServer) newValidatorSetEntry(ctx context.Context, validator stakingTypes.ValidatorI) (*types.ValidatorSetEntry, error) {
	consPubKey, err := validator.ConsPubKey()
	if err != nil {
		return nil, fmt.Errorf("failed to read validator consensus public key: %w", err)
	}

	consPubKeyBytes := consPubKey.Bytes()
	minaAddr, err := q.k.ValidatorGetCosmosToMina(ctx, consPubKeyBytes)
	if err != nil {
		return nil, err
	}

	return &types.ValidatorSetEntry{
		ValidatorCosmosPubKey: consPubKeyBytes,
		ValidatorMinaPubKey:   minaAddr,
		ConsensusPower:        validator.GetConsensusPower(sdk.DefaultPowerReduction),
	}, nil
}
