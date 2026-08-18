package keeper

import (
	"context"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func (k Keeper) InitGenesis(ctx context.Context, state types.GenesisState) error {
	if err := state.Validate(); err != nil {
		return err
	}

	if err := k.Params.Set(ctx, state.Params); err != nil {
		return err
	}
	for _, entry := range state.ProofCounts {
		if err := k.ProofCountByHeight.Set(ctx, entry.Height, entry.Count); err != nil {
			return err
		}
	}
	for _, entry := range state.PendingProofs {
		if err := k.PendingProofs.Set(ctx, types.NewProofStoreKey(entry.ProofKey.SubmissionHeight, entry.ProofKey.IndexInBlock), entry.Proof); err != nil {
			return err
		}
	}
	for _, entry := range state.SeenProofHashes {
		if err := k.SeenProofHashes.Set(ctx, append([]byte(nil), entry.ProofHash...), entry.ProofKey); err != nil {
			return err
		}
	}
	for _, entry := range state.ValidatorSnapshots {
		if err := k.ValidatorSnapshots.Set(ctx, types.NewValidatorSnapshotStoreKey(entry.Height, entry.Validator)); err != nil {
			return err
		}
	}
	for _, entry := range state.ValidatorCounts {
		if err := k.ValidatorCountByHeight.Set(ctx, entry.Height, entry.Count); err != nil {
			return err
		}
	}
	for _, entry := range state.Commitments {
		if err := k.Commitments.Set(ctx, types.NewCommitmentStoreKey(entry.Validator, entry.Height), append([]byte(nil), entry.Commitment...)); err != nil {
			return err
		}
		if err := k.CommitmentsByHeight.Set(ctx, types.NewCommitmentHeightStoreKey(entry.Height, entry.Validator)); err != nil {
			return err
		}
	}
	for _, entry := range state.VerificationVotes {
		if err := k.VerificationVotes.Set(ctx, types.NewVerificationVoteStoreKey(entry.ProofKey.SubmissionHeight, entry.ProofKey.IndexInBlock, entry.Validator), uint32(entry.State)); err != nil {
			return err
		}
	}
	for _, entry := range state.ProofTallies {
		if err := k.ProofTallies.Set(ctx, types.NewProofStoreKey(entry.ProofKey.SubmissionHeight, entry.ProofKey.IndexInBlock), entry.Tally); err != nil {
			return err
		}
	}
	for _, entry := range state.FinalProofResults {
		if err := k.FinalProofResults.Set(ctx, types.NewProofStoreKey(entry.ProofKey.SubmissionHeight, entry.ProofKey.IndexInBlock), entry.Result); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	state := &types.GenesisState{Params: params}
	if err := k.ProofCountByHeight.Walk(ctx, nil, func(height uint64, count uint32) (bool, error) {
		state.ProofCounts = append(state.ProofCounts, types.GenesisProofCount{Height: height, Count: count})
		return false, nil
	}); err != nil {
		return nil, err
	}
	if err := k.PendingProofs.Walk(ctx, nil, func(key types.ProofStoreKey, proof types.ProofRecord) (bool, error) {
		state.PendingProofs = append(state.PendingProofs, types.GenesisPendingProof{ProofKey: types.ProofKey{SubmissionHeight: key.K1(), IndexInBlock: key.K2()}, Proof: proof})
		return false, nil
	}); err != nil {
		return nil, err
	}
	if err := k.SeenProofHashes.Walk(ctx, nil, func(hash []byte, key types.ProofKey) (bool, error) {
		state.SeenProofHashes = append(state.SeenProofHashes, types.GenesisSeenProofHash{ProofHash: append([]byte(nil), hash...), ProofKey: key})
		return false, nil
	}); err != nil {
		return nil, err
	}
	if err := k.ValidatorSnapshots.Walk(ctx, nil, func(key types.ValidatorSnapshotStoreKey) (bool, error) {
		state.ValidatorSnapshots = append(state.ValidatorSnapshots, types.GenesisValidatorSnapshot{Height: key.K1(), Validator: append([]byte(nil), key.K2()...)})
		return false, nil
	}); err != nil {
		return nil, err
	}
	if err := k.ValidatorCountByHeight.Walk(ctx, nil, func(height uint64, count uint32) (bool, error) {
		state.ValidatorCounts = append(state.ValidatorCounts, types.GenesisValidatorCount{Height: height, Count: count})
		return false, nil
	}); err != nil {
		return nil, err
	}
	if err := k.Commitments.Walk(ctx, nil, func(key types.CommitmentStoreKey, commitment []byte) (bool, error) {
		state.Commitments = append(state.Commitments, types.GenesisCommitment{Validator: append([]byte(nil), key.K1()...), Height: key.K2(), Commitment: append([]byte(nil), commitment...)})
		return false, nil
	}); err != nil {
		return nil, err
	}
	if err := k.VerificationVotes.Walk(ctx, nil, func(key types.VerificationVoteStoreKey, vote uint32) (bool, error) {
		state.VerificationVotes = append(state.VerificationVotes, types.GenesisVerificationVote{ProofKey: types.ProofKey{SubmissionHeight: key.K1(), IndexInBlock: key.K2()}, Validator: append([]byte(nil), key.K3()...), State: types.VoteState(vote)})
		return false, nil
	}); err != nil {
		return nil, err
	}
	if err := k.ProofTallies.Walk(ctx, nil, func(key types.ProofStoreKey, tally types.ProofTally) (bool, error) {
		state.ProofTallies = append(state.ProofTallies, types.GenesisProofTally{ProofKey: types.ProofKey{SubmissionHeight: key.K1(), IndexInBlock: key.K2()}, Tally: tally})
		return false, nil
	}); err != nil {
		return nil, err
	}
	if err := k.FinalProofResults.Walk(ctx, nil, func(key types.ProofStoreKey, result types.FinalProofResult) (bool, error) {
		state.FinalProofResults = append(state.FinalProofResults, types.GenesisFinalProofResult{ProofKey: types.ProofKey{SubmissionHeight: key.K1(), IndexInBlock: key.K2()}, Result: result})
		return false, nil
	}); err != nil {
		return nil, err
	}

	return state, nil
}
