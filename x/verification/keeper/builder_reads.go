package keeper

import (
	"bytes"
	"context"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

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
		if len(record.ProofHash) != types.ProofHashSize {
			return nil, types.ErrProofStateCorrupted
		}
		registered, err := k.SeenProofHashes.Get(ctx, record.ProofHash)
		if err != nil || registered.SubmissionHeight != height || registered.IndexInBlock != index {
			return nil, types.ErrProofStateCorrupted
		}
		proofs = append(proofs, types.ProofEntry{
			Key: types.ProofKey{SubmissionHeight: height, IndexInBlock: index},
			Record: types.ProofRecord{
				ProofHash: append([]byte(nil), record.ProofHash...),
				ProofType: record.ProofType,
			},
		})
	}

	return proofs, nil
}

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
