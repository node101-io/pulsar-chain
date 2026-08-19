package keeper

import (
	"context"
	"encoding/hex"
	"strconv"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

// EndBlock finalizes the proof height whose last reveal opportunity has just
// closed, then prunes lifecycle state that can no longer affect consensus. It
// runs after all H+5 revelations have been applied, ensuring the second reveal
// block is a real voting opportunity rather than a premature cutoff.
func (k Keeper) EndBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	cacheCtx, write := sdkCtx.CacheContext()
	if err := k.endBlock(cacheCtx); err != nil {
		return err
	}
	write()

	return nil
}

func (k Keeper) endBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height, err := blockHeight(sdkCtx)
	if err != nil {
		return err
	}
	// Proofs submitted at H finalize at EndBlock(H+5). At this point both
	// commitment opportunities and both active reveal blocks are finished.
	finalRevealOffset := types.VerificationLifetime - 1
	if height >= finalRevealOffset {
		proofHeight := height - finalRevealOffset
		if err := k.FinalizeHeight(ctx, proofHeight, height); err != nil {
			return err
		}
		if err := k.PruneProofHeight(ctx, proofHeight); err != nil {
			return err
		}
	}
	// Commitments remain available through C+3 and are pruned at EndBlock(C+3).
	// The root is no longer useful after its final legal revelation has run.
	if height >= types.CommitmentDeadline {
		if err := k.PruneCommitmentsAtHeight(ctx, height-types.CommitmentDeadline); err != nil {
			return err
		}
	}
	proofsAtCurrentHeight, err := k.ProofCountByHeight.Has(ctx, height)
	if err != nil {
		return err
	}
	// PreBlock may create a snapshot before transactions are known. Remove it
	// when the block ended without any successful proof submission.
	if !proofsAtCurrentHeight {
		snapshotExists, err := k.ValidatorCountByHeight.Has(ctx, height)
		if err != nil {
			return err
		}
		if snapshotExists {
			if err := k.RemoveValidatorSnapshot(ctx, height); err != nil {
				return err
			}
		}
	}

	return nil
}

// FinalizeHeight derives immutable VALID, INVALID, or INCONCLUSIVE results
// from the proof-height snapshot and effective tallies. Inconclusive is a real
// consensus outcome: missing sidecar results, absent validators, and removed
// equivocations are not silently converted into INVALID votes.
func (k Keeper) FinalizeHeight(ctx context.Context, proofHeight, finalizedHeight uint64) error {
	started := telemetry.Now()
	defer telemetry.ModuleMeasureSince(types.ModuleName, started, "finalization", "duration")

	exists, err := k.ProofCountByHeight.Has(ctx, proofHeight)
	if err != nil || !exists {
		return err
	}
	proofCount, err := k.ProofCountByHeight.Get(ctx, proofHeight)
	if err != nil {
		return err
	}
	validatorCountExists, err := k.ValidatorCountByHeight.Has(ctx, proofHeight)
	if err != nil {
		return err
	}
	if !validatorCountExists {
		return types.ErrEmptyValidatorSet
	}
	validatorCount, err := k.ValidatorCountByHeight.Get(ctx, proofHeight)
	if err != nil {
		return err
	}
	threshold, err := types.ComputeThreshold(validatorCount)
	if err != nil {
		return err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	for index := uint32(0); index < proofCount; index++ {
		key := types.NewProofStoreKey(proofHeight, index)
		proof, err := k.PendingProofs.Get(ctx, key)
		if err != nil {
			return errorsmod.Wrap(types.ErrProofStateCorrupted, "missing pending proof")
		}
		if len(proof.ProofHash) != types.ProofHashSize {
			return types.ErrProofStateCorrupted
		}
		tally, err := k.ProofTallies.Get(ctx, key)
		if err != nil {
			return errorsmod.Wrap(types.ErrProofStateCorrupted, "missing proof tally")
		}
		if uint64(tally.TrueVotes)+uint64(tally.FalseVotes) > uint64(validatorCount) {
			return errorsmod.Wrap(types.ErrProofStateCorrupted, "proof tally exceeds validator snapshot")
		}
		// Only one side can reach ceil(2N/3) because effective votes are bounded
		// by the validator snapshot and equivocations count toward neither side.
		// This gives VALID and INVALID symmetric finalization rules.
		status := types.ProofStatus_PROOF_STATUS_INCONCLUSIVE
		if tally.TrueVotes >= threshold {
			status = types.ProofStatus_PROOF_STATUS_VALID
		} else if tally.FalseVotes >= threshold {
			status = types.ProofStatus_PROOF_STATUS_INVALID
		}
		result := types.FinalProofResult{
			ProofHash:              append([]byte(nil), proof.ProofHash...),
			ProofType:              proof.ProofType,
			Status:                 status,
			TrueVotes:              tally.TrueVotes,
			FalseVotes:             tally.FalseVotes,
			EligibleValidatorCount: validatorCount,
			Threshold:              threshold,
			SubmissionHeight:       proofHeight,
			FinalizedHeight:        finalizedHeight,
		}
		if err := k.FinalProofResults.Set(ctx, key, result); err != nil {
			return err
		}
		telemetry.IncrCounter(1, types.ModuleName, "finalization", status.String())
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeProofFinalized,
			sdk.NewAttribute(types.AttributeKeyProofHash, hex.EncodeToString(proof.ProofHash)),
			sdk.NewAttribute(types.AttributeKeySubmissionHeight, strconv.FormatUint(proofHeight, 10)),
			sdk.NewAttribute(types.AttributeKeyIndexInBlock, strconv.FormatUint(uint64(index), 10)),
			sdk.NewAttribute(types.AttributeKeyStatus, status.String()),
			sdk.NewAttribute(types.AttributeKeyTrueVotes, strconv.FormatUint(uint64(tally.TrueVotes), 10)),
			sdk.NewAttribute(types.AttributeKeyFalseVotes, strconv.FormatUint(uint64(tally.FalseVotes), 10)),
			sdk.NewAttribute(types.AttributeKeyValidatorCount, strconv.FormatUint(uint64(validatorCount), 10)),
			sdk.NewAttribute(types.AttributeKeyThreshold, strconv.FormatUint(uint64(threshold), 10)),
		))
	}

	return nil
}

// PruneProofHeight removes active proofs, votes, tallies, counts, and validator
// snapshots after finalization. Final results remain queryable, while permanent
// hash-to-key mappings keep replay protection even after bulky lifecycle state
// is gone.
func (k Keeper) PruneProofHeight(ctx context.Context, height uint64) error {
	exists, err := k.ProofCountByHeight.Has(ctx, height)
	if err != nil || !exists {
		return err
	}
	proofCount, err := k.ProofCountByHeight.Get(ctx, height)
	if err != nil {
		return err
	}
	for index := uint32(0); index < proofCount; index++ {
		key := types.NewProofStoreKey(height, index)
		voteKeys := make([]types.VerificationVoteStoreKey, 0)
		rangeByProof := collections.NewSuperPrefixedTripleRange[uint64, uint32, []byte](height, index)
		if err := k.VerificationVotes.Walk(ctx, rangeByProof, func(key types.VerificationVoteStoreKey, _ uint32) (bool, error) {
			voteKeys = append(voteKeys, key)
			return false, nil
		}); err != nil {
			return err
		}
		for _, voteKey := range voteKeys {
			if err := k.VerificationVotes.Remove(ctx, voteKey); err != nil {
				return err
			}
		}
		if err := k.PendingProofs.Remove(ctx, key); err != nil {
			return err
		}
		if err := k.ProofTallies.Remove(ctx, key); err != nil {
			return err
		}
	}

	snapshotKeys := make([]types.ValidatorSnapshotStoreKey, 0)
	snapshotRange := collections.NewPrefixedPairRange[uint64, []byte](height)
	if err := k.ValidatorSnapshots.Walk(ctx, snapshotRange, func(key types.ValidatorSnapshotStoreKey) (bool, error) {
		snapshotKeys = append(snapshotKeys, key)
		return false, nil
	}); err != nil {
		return err
	}
	for _, key := range snapshotKeys {
		if err := k.ValidatorSnapshots.Remove(ctx, key); err != nil {
			return err
		}
	}
	if err := k.ValidatorCountByHeight.Remove(ctx, height); err != nil {
		return err
	}

	return k.ProofCountByHeight.Remove(ctx, height)
}

// PruneCommitmentsAtHeight removes every commitment at one height by walking
// the reverse index and checks that the primary index is still consistent. The
// reverse index avoids scanning commitments for every validator at every block.
func (k Keeper) PruneCommitmentsAtHeight(ctx context.Context, height uint64) error {
	keys := make([]types.CommitmentHeightStoreKey, 0)
	rangeByHeight := collections.NewPrefixedPairRange[uint64, []byte](height)
	if err := k.CommitmentsByHeight.Walk(ctx, rangeByHeight, func(key types.CommitmentHeightStoreKey) (bool, error) {
		keys = append(keys, key)
		return false, nil
	}); err != nil {
		return err
	}
	for _, reverseKey := range keys {
		primaryKey := types.NewCommitmentStoreKey(reverseKey.K2(), height)
		exists, err := k.Commitments.Has(ctx, primaryKey)
		if err != nil {
			return err
		}
		if !exists {
			return types.ErrProofStateCorrupted
		}
		if err := k.Commitments.Remove(ctx, primaryKey); err != nil {
			return err
		}
		if err := k.CommitmentsByHeight.Remove(ctx, reverseKey); err != nil {
			return err
		}
	}

	return nil
}
