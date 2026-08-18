package keeper

import (
	"context"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func (k Keeper) submitCommitment(ctx context.Context, validator []byte, height uint64, commitment []byte) error {
	if err := k.validateSubmitCommitment(ctx, validator, height, commitment); err != nil {
		return err
	}
	key := types.NewCommitmentStoreKey(validator, height)
	if err := k.Commitments.Set(ctx, key, append([]byte(nil), commitment...)); err != nil {
		return err
	}

	return k.CommitmentsByHeight.Set(ctx, types.NewCommitmentHeightStoreKey(height, validator))
}

func (k Keeper) validateSubmitCommitment(ctx context.Context, validator []byte, height uint64, commitment []byte) error {
	if len(commitment) != types.CommitmentHashSize {
		return types.ErrInvalidCommitmentLength
	}
	leftHeight, rightHeight, err := types.CommitmentProofHeights(height)
	if err != nil {
		return err
	}
	leftEligible, err := k.IsValidatorEligible(ctx, leftHeight, validator)
	if err != nil {
		return err
	}
	rightEligible, err := k.IsValidatorEligible(ctx, rightHeight, validator)
	if err != nil {
		return err
	}
	if !leftEligible && !rightEligible {
		return types.ErrInvalidValidator
	}

	exists, err := k.Commitments.Has(ctx, types.NewCommitmentStoreKey(validator, height))
	if err != nil {
		return err
	}
	if exists {
		return types.ErrCommitmentAlreadyExists
	}

	return nil
}
