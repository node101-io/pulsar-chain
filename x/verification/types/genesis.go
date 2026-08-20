package types

import (
	"fmt"
	"math"
)

// DefaultGenesis returns an empty module state with production defaults.
func DefaultGenesis() *GenesisState {
	return &GenesisState{Params: DefaultParams()}
}

// Validate reconstructs the relationships between every consensus collection.
// Genesis bypasses normal handlers, so imported power snapshots, votes, tallies,
// final results, and the permanent hash registry must be mutually derivable.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}

	proofCounts := make(map[uint64]uint32, len(gs.ProofCounts))
	for _, entry := range gs.ProofCounts {
		if entry.Count == 0 || entry.Count > gs.Params.MaxProofsPerBlock {
			return fmt.Errorf("invalid proof count at height %d", entry.Height)
		}
		if _, duplicate := proofCounts[entry.Height]; duplicate {
			return fmt.Errorf("duplicate proof count at height %d", entry.Height)
		}
		proofCounts[entry.Height] = entry.Count
	}

	pending := make(map[string]ProofRecord, len(gs.PendingProofs))
	for _, entry := range gs.PendingProofs {
		key := genesisProofKey(entry.ProofKey.SubmissionHeight, entry.ProofKey.IndexInBlock)
		count, ok := proofCounts[entry.ProofKey.SubmissionHeight]
		if !ok || entry.ProofKey.IndexInBlock >= count || len(entry.Proof.ProofHash) != ProofHashSize {
			return fmt.Errorf("invalid pending proof %d/%d", entry.ProofKey.SubmissionHeight, entry.ProofKey.IndexInBlock)
		}
		if _, duplicate := pending[key]; duplicate {
			return fmt.Errorf("duplicate pending proof %d/%d", entry.ProofKey.SubmissionHeight, entry.ProofKey.IndexInBlock)
		}
		pending[key] = entry.Proof
	}
	for height, count := range proofCounts {
		for index := uint32(0); index < count; index++ {
			if _, ok := pending[genesisProofKey(height, index)]; !ok {
				return fmt.Errorf("missing pending proof %d/%d", height, index)
			}
		}
	}

	// Every active height owns one complete positive-power snapshot. Finalized
	// heights have already pruned it, and heights without proofs must not retain
	// orphan power records.
	totalPowers := make(map[uint64]int64, len(gs.TotalVotingPowers))
	for _, entry := range gs.TotalVotingPowers {
		if _, active := proofCounts[entry.Height]; !active {
			return fmt.Errorf("orphan total voting power at height %d", entry.Height)
		}
		if !IsValidTotalVotingPower(entry.TotalVotingPower) {
			return fmt.Errorf("invalid total voting power at height %d", entry.Height)
		}
		if _, duplicate := totalPowers[entry.Height]; duplicate {
			return fmt.Errorf("duplicate total voting power at height %d", entry.Height)
		}
		totalPowers[entry.Height] = entry.TotalVotingPower
	}
	validatorPowers := make(map[string]int64, len(gs.ValidatorPowers))
	powerSums := make(map[uint64]int64, len(proofCounts))
	for _, entry := range gs.ValidatorPowers {
		totalPower, active := totalPowers[entry.Height]
		if !active {
			return fmt.Errorf("orphan validator power at height %d", entry.Height)
		}
		if len(entry.Validator) == 0 {
			return ErrInvalidValidator
		}
		if entry.VotingPower <= 0 {
			return fmt.Errorf("non-positive validator power at height %d", entry.Height)
		}
		key := genesisValidatorKey(entry.Height, entry.Validator)
		if _, duplicate := validatorPowers[key]; duplicate {
			return fmt.Errorf("duplicate validator power %s", key)
		}
		sum := powerSums[entry.Height]
		if entry.VotingPower > totalPower-sum {
			return fmt.Errorf("validator powers exceed total at height %d", entry.Height)
		}
		validatorPowers[key] = entry.VotingPower
		powerSums[entry.Height] = sum + entry.VotingPower
	}
	for height := range proofCounts {
		totalPower, ok := totalPowers[height]
		if !ok || powerSums[height] != totalPower {
			return fmt.Errorf("incomplete validator power snapshot at height %d", height)
		}
	}

	// The permanent registry must remain one-to-one after pending state is
	// pruned, otherwise replay protection and hash lookup become ambiguous.
	seenHashes := make(map[string]ProofKey, len(gs.SeenProofHashes))
	seenKeys := make(map[string]struct{}, len(gs.SeenProofHashes))
	for _, entry := range gs.SeenProofHashes {
		if len(entry.ProofHash) != ProofHashSize {
			return ErrInvalidProofHash
		}
		hashKey := string(entry.ProofHash)
		proofKey := genesisProofKey(entry.ProofKey.SubmissionHeight, entry.ProofKey.IndexInBlock)
		if _, duplicate := seenHashes[hashKey]; duplicate {
			return ErrDuplicateProof
		}
		if _, duplicate := seenKeys[proofKey]; duplicate {
			return fmt.Errorf("duplicate proof key in hash registry")
		}
		seenHashes[hashKey] = entry.ProofKey
		seenKeys[proofKey] = struct{}{}
	}
	for key, proof := range pending {
		if seen, ok := seenHashes[string(proof.ProofHash)]; !ok ||
			genesisProofKey(seen.SubmissionHeight, seen.IndexInBlock) != key {
			return fmt.Errorf("pending proof missing permanent hash mapping")
		}
	}

	commitments := make(map[string]struct{}, len(gs.Commitments))
	for _, entry := range gs.Commitments {
		if len(entry.Validator) == 0 || len(entry.Commitment) != CommitmentHashSize {
			return ErrInvalidCommitmentLength
		}
		key := fmt.Sprintf("%x/%d", entry.Validator, entry.Height)
		if _, duplicate := commitments[key]; duplicate {
			return ErrCommitmentAlreadyExists
		}
		commitments[key] = struct{}{}
	}

	tallies := make(map[string]ProofTally, len(gs.ProofTallies))
	for _, entry := range gs.ProofTallies {
		key := genesisProofKey(entry.ProofKey.SubmissionHeight, entry.ProofKey.IndexInBlock)
		if _, ok := pending[key]; !ok {
			return fmt.Errorf("tally references nonexistent pending proof")
		}
		if _, duplicate := tallies[key]; duplicate {
			return fmt.Errorf("duplicate proof tally")
		}
		totalPower := totalPowers[entry.ProofKey.SubmissionHeight]
		if !validGenesisPowerTally(entry.Tally, totalPower) {
			return fmt.Errorf("proof tally exceeds total voting power")
		}
		tallies[key] = entry.Tally
	}
	if len(tallies) != len(pending) {
		return fmt.Errorf("every pending proof must have one tally")
	}

	// Recompute tallies from effective votes using each validator's historical
	// power. Equivocated validators deliberately contribute to neither side.
	votes := make(map[string]struct{}, len(gs.VerificationVotes))
	derivedTallies := make(map[string]ProofTally, len(gs.ProofTallies))
	for _, entry := range gs.VerificationVotes {
		key := genesisProofKey(entry.ProofKey.SubmissionHeight, entry.ProofKey.IndexInBlock)
		if _, ok := pending[key]; !ok || len(entry.Validator) == 0 ||
			entry.State <= VoteState_VOTE_STATE_NONE || entry.State > VoteState_VOTE_STATE_EQUIVOCATED {
			return fmt.Errorf("invalid verification vote")
		}
		power, ok := validatorPowers[genesisValidatorKey(entry.ProofKey.SubmissionHeight, entry.Validator)]
		if !ok {
			return fmt.Errorf("verification vote is not from an eligible validator")
		}
		voteKey := fmt.Sprintf("%d/%d/%x", entry.ProofKey.SubmissionHeight, entry.ProofKey.IndexInBlock, entry.Validator)
		if _, duplicate := votes[voteKey]; duplicate {
			return fmt.Errorf("duplicate verification vote")
		}
		votes[voteKey] = struct{}{}
		derived := derivedTallies[key]
		switch entry.State {
		case VoteState_VOTE_STATE_TRUE:
			derived.ValidVotingPower += power
		case VoteState_VOTE_STATE_FALSE:
			derived.InvalidVotingPower += power
		}
		if !validGenesisPowerTally(derived, totalPowers[entry.ProofKey.SubmissionHeight]) {
			return fmt.Errorf("derived proof tally exceeds total voting power")
		}
		derivedTallies[key] = derived
	}
	for key, tally := range tallies {
		if derivedTallies[key] != tally {
			return fmt.Errorf("proof tally does not match effective votes")
		}
	}

	// Final results are self-contained because their active snapshots were
	// pruned at H+5. Reapply the same strict threshold rule used by EndBlock.
	finals := make(map[string]struct{}, len(gs.FinalProofResults))
	for _, entry := range gs.FinalProofResults {
		key := genesisProofKey(entry.ProofKey.SubmissionHeight, entry.ProofKey.IndexInBlock)
		if _, duplicate := finals[key]; duplicate {
			return fmt.Errorf("duplicate final proof result")
		}
		_, activeHeight := proofCounts[entry.ProofKey.SubmissionHeight]
		if _, active := pending[key]; active || activeHeight || entry.ProofKey.IndexInBlock >= MaxVoteIndexExclusive ||
			len(entry.Result.ProofHash) != ProofHashSize ||
			entry.Result.SubmissionHeight != entry.ProofKey.SubmissionHeight ||
			entry.Result.SubmissionHeight > math.MaxUint64-(VerificationLifetime-1) ||
			entry.Result.FinalizedHeight != entry.ProofKey.SubmissionHeight+VerificationLifetime-1 ||
			entry.Result.Status < ProofStatus_PROOF_STATUS_VALID ||
			entry.Result.Status > ProofStatus_PROOF_STATUS_INCONCLUSIVE {
			return fmt.Errorf("invalid final proof result")
		}
		threshold, err := ComputeVotingPowerThreshold(entry.Result.TotalVotingPower)
		if err != nil || threshold != entry.Result.VotingPowerThreshold {
			return fmt.Errorf("invalid final proof threshold")
		}
		finalTally := ProofTally{
			ValidVotingPower:   entry.Result.ValidVotingPower,
			InvalidVotingPower: entry.Result.InvalidVotingPower,
		}
		if !validGenesisPowerTally(finalTally, entry.Result.TotalVotingPower) {
			return fmt.Errorf("final proof tally exceeds total voting power")
		}
		expected := ProofStatus_PROOF_STATUS_INCONCLUSIVE
		if HasTwoThirdsMajority(entry.Result.ValidVotingPower, entry.Result.TotalVotingPower) {
			expected = ProofStatus_PROOF_STATUS_VALID
		} else if HasTwoThirdsMajority(entry.Result.InvalidVotingPower, entry.Result.TotalVotingPower) {
			expected = ProofStatus_PROOF_STATUS_INVALID
		}
		if entry.Result.Status != expected {
			return fmt.Errorf("final proof status does not match voting power")
		}
		seen, ok := seenHashes[string(entry.Result.ProofHash)]
		if !ok || seen.SubmissionHeight != entry.ProofKey.SubmissionHeight || seen.IndexInBlock != entry.ProofKey.IndexInBlock {
			return fmt.Errorf("final proof missing permanent hash mapping")
		}
		finals[key] = struct{}{}
	}
	if len(seenHashes) != len(pending)+len(finals) {
		return fmt.Errorf("hash registry contains an unreferenced proof")
	}

	return nil
}

func validGenesisPowerTally(tally ProofTally, totalPower int64) bool {
	return IsValidTotalVotingPower(totalPower) &&
		tally.ValidVotingPower >= 0 && tally.InvalidVotingPower >= 0 &&
		tally.ValidVotingPower <= totalPower-tally.InvalidVotingPower
}

func genesisProofKey(height uint64, index uint32) string {
	return fmt.Sprintf("%d/%d", height, index)
}

func genesisValidatorKey(height uint64, validator []byte) string {
	return fmt.Sprintf("%d/%x", height, validator)
}
