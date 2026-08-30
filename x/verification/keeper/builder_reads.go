package keeper

import (
	"bytes"
	"context"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

// GetProofsAtHeight returns the complete, index-ordered proof batch used by the
// validator-local builder. The sidecar sees only verification IDs, while this
// method preserves the canonical ProofKey mapping needed to encode one-byte vote
// indices. Gaps or stale permanent ID mappings are rejected as corruption so
// the builder never commits a result to the wrong proof.
func (k Keeper) GetProofsAtHeight(ctx context.Context, height uint64) ([]types.ProofEntry, error) {
	found, err := k.ProofCountByHeight.Has(ctx, height)
	if err != nil || !found {
		return nil, err
	}
	count, err := k.ProofCountByHeight.Get(ctx, height)
	if err != nil {
		return nil, err
	}
	if count == 0 || count > types.MaxVoteIndexExclusive {
		return nil, types.ErrProofStateCorrupted
	}

	proofs := make([]types.ProofEntry, 0, count)
	for index := uint32(0); index < count; index++ {
		storeKey := types.NewProofStoreKey(height, index)
		record, err := k.PendingProofs.Get(ctx, storeKey)
		if err != nil {
			return nil, types.ErrProofStateCorrupted
		}
		if err := types.ValidateProofRecord(record); err != nil {
			return nil, types.ErrProofStateCorrupted
		}
		registered, err := k.SeenVerificationIDs.Get(ctx, record.VerificationId)
		if err != nil || registered.SubmissionHeight != height || registered.IndexInBlock != index {
			return nil, types.ErrProofStateCorrupted
		}
		proofs = append(proofs, types.ProofEntry{
			Key: types.ProofKey{SubmissionHeight: height, IndexInBlock: index},
			Record: types.ProofRecord{
				ProofHash:           append([]byte(nil), record.ProofHash...),
				ProofType:           record.ProofType,
				PublicInputsHash:    append([]byte(nil), record.PublicInputsHash...),
				VerificationKeyHash: append([]byte(nil), record.VerificationKeyHash...),
				VerificationId:      append([]byte(nil), record.VerificationId...),
			},
		})
	}

	return proofs, nil
}

// GetCommitment returns a defensive copy of one validator commitment. The local
// builder uses it to confirm that a persisted root actually reached consensus
// before revealing private salts or excluding already committed proofs from the
// H+3 retry. The bool distinguishes a normal missing root from storage failure.
func (k Keeper) GetCommitment(
	ctx context.Context,
	validator []byte,
	commitmentHeight uint64,
) ([]byte, bool, error) {
	key := types.NewCommitmentStoreKey(validator, commitmentHeight)
	found, err := k.Commitments.Has(ctx, key)
	if err != nil || !found {
		return nil, found, err
	}
	root, err := k.Commitments.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}
	if len(root) != types.CommitmentHashSize {
		return nil, false, types.ErrProofStateCorrupted
	}

	return bytes.Clone(root), true, nil
}
