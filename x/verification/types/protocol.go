package types

import (
	"crypto/sha256"
	"encoding/binary"
	"math"

	comettypes "github.com/cometbft/cometbft/types"
)

// The verification lifecycle for proofs submitted in block H is:
//
//   - H: the chain registers verification descriptors and freezes historical voting power.
//   - H+2: the proof can appear in the right leaf of a new commitment.
//   - H+3: the same proof height can appear in the next commitment's left leaf.
//   - H+4 and H+5: committed values can be revealed and counted.
//   - EndBlock(H+5): the chain finalizes and prunes temporary lifecycle state.
//
// The overlapping H+2/H+3 slots give asynchronous sidecar verification one
// extra block to finish. They are not intended to commit the same proof twice;
// the validator-local builder excludes results that were already committed.

// CommitmentProofHeights returns the two proof heights covered by a
// commitment created at commitmentHeight. A commitment at C covers C-3 on
// the left and C-2 on the right. This two-leaf layout is what creates the two
// commitment opportunities for each proof height without growing the root.
func CommitmentProofHeights(commitmentHeight uint64) (uint64, uint64, error) {
	if commitmentHeight < CommitmentDeadline {
		return 0, 0, ErrInvalidCommitmentHeight
	}

	return commitmentHeight - CommitmentDeadline, commitmentHeight - (CommitmentDeadline - 1), nil
}

// ValidateRevelationCommitmentHeight checks that a retained commitment is in
// its reveal window. A commitment created at C may be revealed at C+1, C+2,
// or C+3. The bounded window prevents old commitments from being reopened
// after the related proofs have finalized.
func ValidateRevelationCommitmentHeight(currentHeight, commitmentHeight uint64) error {
	if currentHeight < commitmentHeight {
		return ErrInvalidCommitmentHeight
	}
	delta := currentHeight - commitmentHeight
	if delta < 1 || delta > CommitmentDeadline {
		return ErrInvalidCommitmentHeight
	}

	return nil
}

// GetLeafTiming classifies a proof leaf relative to its two-block reveal
// window. Expired values may still be supplied as hashes to reconstruct a
// commitment root, but their votes no longer affect the tally. This separates
// authentication of the two-leaf root from eligibility to add a late vote.
func GetLeafTiming(currentHeight, proofHeight uint64) LeafTiming {
	if proofHeight > math.MaxUint64-(VerificationLifetime-1) {
		return LeafTooEarly
	}
	firstReveal := proofHeight + VerificationLifetime - RevealDeadline
	lastReveal := proofHeight + VerificationLifetime - 1

	if currentHeight < firstReveal {
		return LeafTooEarly
	}
	if currentHeight <= lastReveal {
		return LeafActive
	}

	return LeafExpired
}

// ValidateCanonicalVotes enforces the byte-level vote encoding contract:
// indices fit in one byte, appear once, and are strictly increasing. Without
// canonical ordering, the same logical vote set could produce different leaf
// hashes on different validators.
func ValidateCanonicalVotes(votes []ProofVote) error {
	if len(votes) > MaxVoteIndexExclusive {
		return ErrTooManyVotes
	}

	var previous uint32
	for i, vote := range votes {
		if vote.IndexInBlock >= MaxVoteIndexExclusive {
			return ErrInvalidVoteIndex
		}
		if i > 0 {
			if vote.IndexInBlock == previous {
				return ErrDuplicateVoteIndex
			}
			if vote.IndexInBlock < previous {
				return ErrNonCanonicalVoteOrdering
			}
		}
		previous = vote.IndexInBlock
	}

	return nil
}

// EncodeVotes serializes a canonical vote list as a big-endian uint16 count
// followed by one index byte and one result byte per vote. The explicit count
// prevents ambiguous concatenations and keeps root reconstruction independent
// of protobuf encoding details.
func EncodeVotes(votes []ProofVote) ([]byte, error) {
	if err := ValidateCanonicalVotes(votes); err != nil {
		return nil, err
	}

	encoded := make([]byte, 2+len(votes)*2)
	binary.BigEndian.PutUint16(encoded[:2], uint16(len(votes)))
	for i, vote := range votes {
		offset := 2 + i*2
		encoded[offset] = byte(vote.IndexInBlock)
		if vote.Result {
			encoded[offset+1] = 1
		}
	}

	return encoded, nil
}

// ComputeLeafHash commits to a private salt and the canonical vote encoding.
// The salt hides the vote list until reveal time and makes independently built
// commitments different even when validators chose the same results. Only the
// first LeafHashSize bytes are used by this protocol version.
func ComputeLeafHash(salt []byte, votes []ProofVote) ([LeafHashSize]byte, error) {
	var out [LeafHashSize]byte
	if len(salt) != SaltSize {
		return out, ErrInvalidSaltLength
	}
	encoded, err := EncodeVotes(votes)
	if err != nil {
		return out, err
	}
	preimage := make([]byte, 0, SaltSize+len(encoded))
	preimage = append(preimage, salt...)
	preimage = append(preimage, encoded...)
	digest := sha256.Sum256(preimage)
	copy(out[:], digest[:LeafHashSize])

	return out, nil
}

// ComputeCommitmentRoot combines the left and right leaf hashes in that exact
// order. Swapping leaves changes the commitment because each position refers
// to a different proof height.
func ComputeCommitmentRoot(left, right [LeafHashSize]byte) [CommitmentHashSize]byte {
	// TODO(security): bind this preimage to a domain separator, chain ID,
	// commitment height, and validator address in a future protocol version.
	var preimage [LeafHashSize * 2]byte
	copy(preimage[:LeafHashSize], left[:])
	copy(preimage[LeafHashSize:], right[:])
	digest := sha256.Sum256(preimage[:])
	var out [CommitmentHashSize]byte
	copy(out[:], digest[:CommitmentHashSize])

	return out
}

// IsValidTotalVotingPower reports whether a snapshot denominator is usable by
// both verification and CometBFT consensus arithmetic.
func IsValidTotalVotingPower(totalPower int64) bool {
	return totalPower > 0 && totalPower <= comettypes.MaxTotalVotingPower
}

// HasTwoThirdsMajority applies CometBFT's strict greater-than-two-thirds rule.
// Verification uses this independently from the ABCI Mina quorum, whose
// existing greater-than-or-equal behavior is intentionally unchanged.
func HasTwoThirdsMajority(power, totalPower int64) bool {
	if !IsValidTotalVotingPower(totalPower) || power < 0 || power > totalPower {
		return false
	}
	return power > totalPower*2/3
}

// ComputeVotingPowerThreshold returns the smallest power that satisfies the
// strict majority rule and is stored as human-readable final-result metadata.
func ComputeVotingPowerThreshold(totalPower int64) (int64, error) {
	if totalPower <= 0 {
		return 0, ErrEmptyValidatorSet
	}
	if totalPower > comettypes.MaxTotalVotingPower {
		return 0, ErrProofStateCorrupted
	}
	return totalPower*2/3 + 1, nil
}
