package keeper

import (
	"context"
	"math"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func (k Keeper) CreateValidatorSnapshot(ctx context.Context, height uint64) error {
	exists, err := k.ValidatorCountByHeight.Has(ctx, height)
	if err != nil {
		return err
	}
	if exists {
		count, err := k.ValidatorCountByHeight.Get(ctx, height)
		if err != nil {
			return err
		}
		if count == 0 {
			return types.ErrProofStateCorrupted
		}
		return nil
	}

	validators, err := k.stakingKeeper.GetLastValidators(ctx)
	if err != nil {
		return err
	}
	if len(validators) == 0 {
		return types.ErrEmptyValidatorSet
	}
	if len(validators) > math.MaxUint32 {
		return errorsmod.Wrap(types.ErrProofStateCorrupted, "validator set exceeds uint32")
	}

	addresses := make([][]byte, 0, len(validators))
	seen := make(map[string]struct{}, len(validators))
	validatorCodec := k.stakingKeeper.ValidatorAddressCodec()
	for _, validator := range validators {
		address, err := validatorCodec.StringToBytes(validator.GetOperator())
		if err != nil {
			return errorsmod.Wrap(types.ErrInvalidValidator, "invalid snapshot validator address")
		}
		key := string(address)
		if _, exists := seen[key]; exists {
			return errorsmod.Wrap(types.ErrProofStateCorrupted, "duplicate snapshot validator")
		}
		seen[key] = struct{}{}
		addresses = append(addresses, append([]byte(nil), address...))
	}

	for _, address := range addresses {
		if err := k.ValidatorSnapshots.Set(ctx, types.NewValidatorSnapshotStoreKey(height, address)); err != nil {
			return err
		}
	}

	return k.ValidatorCountByHeight.Set(ctx, height, uint32(len(addresses)))
}

func (k Keeper) IsValidatorEligible(ctx context.Context, proofHeight uint64, validator []byte) (bool, error) {
	return k.ValidatorSnapshots.Has(ctx, types.NewValidatorSnapshotStoreKey(proofHeight, validator))
}

func (k Keeper) RemoveValidatorSnapshot(ctx context.Context, height uint64) error {
	keys := make([]types.ValidatorSnapshotStoreKey, 0)
	rangeByHeight := collections.NewPrefixedPairRange[uint64, []byte](height)
	if err := k.ValidatorSnapshots.Walk(ctx, rangeByHeight, func(key types.ValidatorSnapshotStoreKey) (bool, error) {
		keys = append(keys, key)
		return false, nil
	}); err != nil {
		return err
	}
	for _, key := range keys {
		if err := k.ValidatorSnapshots.Remove(ctx, key); err != nil {
			return err
		}
	}
	return k.ValidatorCountByHeight.Remove(ctx, height)
}
