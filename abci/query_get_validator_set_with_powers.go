package abci

import (
	"bytes"
	"context"
	"fmt"
	"sort"

	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *ABCIHandler) GetValidatorSetWithPowers(ctx context.Context, req *QueryGetValidatorSetWithPowersRequest) (*QueryGetValidatorSetWithPowersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.BlockHeight <= 0 {
		return nil, status.Error(codes.InvalidArgument, "block height must be positive")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentBlockHeight := sdkCtx.BlockHeight()

	if req.BlockHeight > currentBlockHeight {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"requested block height %d is greater than current block height %d",
			req.BlockHeight,
			currentBlockHeight,
		)
	}

	if currentBlockHeight == req.BlockHeight {
		var validatorSet []*ValidatorEntry

		var iterErr error

		err := h.stakingKeeper.IterateLastValidators(ctx, func(index int64, validator stakingTypes.ValidatorI) bool {
			entry, err := h.newValidatorSetEntry(validator)
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

		if err := sortValidatorsByPowerQuery(validatorSet); err != nil {
			return nil, err
		}

		return &QueryGetValidatorSetWithPowersResponse{
			Validator: validatorSet,
		}, nil
	}

	historicalData, err := h.stakingKeeper.GetHistoricalInfo(ctx, req.BlockHeight)
	if err != nil {
		return nil, err
	}

	validatorSet := make([]*ValidatorEntry, 0, len(historicalData.Valset))
	for _, validator := range historicalData.Valset {
		entry, err := h.newValidatorSetEntry(validator)
		if err != nil {
			return nil, err
		}

		validatorSet = append(validatorSet, entry)
	}

	return &QueryGetValidatorSetWithPowersResponse{
		Validator: validatorSet,
	}, nil
}

func sortValidatorsByPowerQuery(validators []*ValidatorEntry) error {

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

func (h *ABCIHandler) newValidatorSetEntry(validator stakingTypes.ValidatorI) (*ValidatorEntry, error) {
	consPubKey, err := validator.ConsPubKey()
	if err != nil {
		return nil, fmt.Errorf("failed to read validator consensus public key: %w", err)
	}

	return &ValidatorEntry{
		ValidatorCosmosPubKey: consPubKey.Bytes(),
		ConsensusPower:        validator.GetConsensusPower(sdk.DefaultPowerReduction),
	}, nil
}
