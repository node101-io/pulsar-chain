package keeper

import (
	"bytes"
	"context"
	"fmt"
	"sort"

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

	var valInfo []stakingTypes.ValidatorI

	if sdkCtx.BlockHeight() == req.BlockHeight {
		err := q.k.stakingKeeper.IterateLastValidators(ctx, func(index int64, validator stakingTypes.ValidatorI) (stop bool) {
			valInfo = append(valInfo, validator)
			return false
		})

		if err != nil {
			return nil, err
		}

		if err := sortValidatorsByPower(valInfo); err != nil {
			return nil, err
		}

		sortedValidatorSet, err := q.buildValidatorSetEntries(ctx, valInfo)
		if err != nil {
			return nil, err
		}

		return &types.QueryGetValidatorSetByHeightResponse{
			Validators: sortedValidatorSet,
		}, nil
	}

	historicalData, err := q.k.stakingKeeper.GetHistoricalInfo(ctx, req.BlockHeight)
	if err != nil {
		return nil, err
	}
	for _, validator := range historicalData.Valset {
		valInfo = append(valInfo, validator)
	}

	if err := sortValidatorsByPower(valInfo); err != nil {
		return nil, err
	}

	sortedValidatorSet, err := q.buildValidatorSetEntries(ctx, valInfo)
	if err != nil {
		return nil, err
	}

	return &types.QueryGetValidatorSetByHeightResponse{
		Validators: sortedValidatorSet,
	}, nil
}

func sortValidatorsByPower(validators []stakingTypes.ValidatorI) error {
	type validatorSortEntry struct {
		validator        stakingTypes.ValidatorI
		consensusAddress []byte
		consensusPower   int64
	}

	entries := make([]validatorSortEntry, 0, len(validators))
	for _, validator := range validators {
		consAddr, err := validator.GetConsAddr()
		if err != nil {
			return fmt.Errorf("failed to read validator consensus address: %w", err)
		}

		entries = append(entries, validatorSortEntry{
			validator:        validator,
			consensusAddress: consAddr,
			consensusPower:   validator.GetConsensusPower(sdk.DefaultPowerReduction),
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].consensusPower == entries[j].consensusPower {
			return bytes.Compare(entries[i].consensusAddress, entries[j].consensusAddress) < 0
		}

		return entries[i].consensusPower > entries[j].consensusPower
	})

	for i, entry := range entries {
		validators[i] = entry.validator
	}

	return nil
}

func (q queryServer) buildValidatorSetEntries(ctx context.Context, validators []stakingTypes.ValidatorI) ([]*types.ValidatorSetEntry, error) {
	entries := make([]*types.ValidatorSetEntry, 0, len(validators))

	for _, validator := range validators {
		consPubKey, err := validator.ConsPubKey()
		if err != nil {
			return nil, fmt.Errorf("failed to read validator consensus public key: %w", err)
		}

		minaAddr, err := q.k.ValidatorGetCosmosToMina(ctx, consPubKey.Bytes())
		if err != nil {
			return nil, err
		}

		entries = append(entries, &types.ValidatorSetEntry{
			ValidatorCosmosPubKey: consPubKey.Bytes(),
			ValidatorMinaPubKey:   minaAddr,
			ConsensusPower:        validator.GetConsensusPower(sdk.DefaultPowerReduction),
		})
	}

	return entries, nil
}
