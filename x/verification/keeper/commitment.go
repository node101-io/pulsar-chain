package keeper

import (
	"context"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

// submitCommitment writes both the primary record and its pruning index after
// all authorization and duplicate checks have passed. The root is opaque at
// this stage; correctness is proven later by reconstructing it from revealed
// leaves.
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

// validateSubmitCommitment accepts a validator that was eligible for either
// proof height covered by this commitment. The authenticated validator
// identity comes from CometBFT, not from payload data. Eligibility for either
// leaf is sufficient because one of the two covered blocks may contain no
// proof or may have used a different historical power snapshot. Duplicate
// protection is scoped to the validator-height slot: a validator may publish
// only one root at C. Random salts make independently matching roots
// impractical, and copying another validator's opaque root cannot produce a
// valid later revelation without that validator's private leaf preimages.
func (k Keeper) validateSubmitCommitment(ctx context.Context, validator []byte, height uint64, commitment []byte) error {
	if len(commitment) != types.CommitmentHashSize {
		return types.ErrInvalidCommitmentLength
	}
	leftHeight, rightHeight, err := types.CommitmentProofHeights(height)
	if err != nil {
		return err
	}
	leftEligible, err := k.hasVotingPowerAtProofHeight(ctx, leftHeight, validator)
	if err != nil {
		return err
	}
	rightEligible, err := k.hasVotingPowerAtProofHeight(ctx, rightHeight, validator)
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

// hasVotingPowerAtProofHeight treats a height without proofs as an empty leaf,
// while requiring a complete, valid power snapshot for every proof-bearing
// height. This lets a two-height commitment cover one empty block safely.
func (k Keeper) hasVotingPowerAtProofHeight(ctx context.Context, height uint64, validator []byte) (bool, error) {
	hasProofs, err := k.ProofCountByHeight.Has(ctx, height)
	if err != nil || !hasProofs {
		return false, err
	}
	totalExists, err := k.TotalVotingPowerByHeight.Has(ctx, height)
	if err != nil {
		return false, err
	}
	if !totalExists {
		return false, types.ErrProofStateCorrupted
	}
	totalPower, err := k.TotalVotingPowerByHeight.Get(ctx, height)
	if err != nil {
		return false, err
	}
	if !types.IsValidTotalVotingPower(totalPower) {
		return false, types.ErrProofStateCorrupted
	}
	powerKey := types.NewValidatorPowerStoreKey(height, validator)
	exists, err := k.ValidatorPowers.Has(ctx, powerKey)
	if err != nil || !exists {
		return false, err
	}
	power, err := k.ValidatorPowers.Get(ctx, powerKey)
	if err != nil {
		return false, err
	}
	if power <= 0 || power > totalPower {
		return false, types.ErrProofStateCorrupted
	}
	return true, nil
}
