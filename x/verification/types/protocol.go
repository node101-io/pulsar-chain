package types

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

func CommitmentProofHeights(commitmentHeight uint64) (uint64, uint64, error) {
	if commitmentHeight < CommitmentDeadline {
		return 0, 0, ErrInvalidCommitmentHeight
	}

	return commitmentHeight - CommitmentDeadline, commitmentHeight - (CommitmentDeadline - 1), nil
}

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

func ComputeThreshold(validatorCount uint32) (uint32, error) {
	if validatorCount == 0 {
		return 0, ErrEmptyValidatorSet
	}

	return uint32((2*uint64(validatorCount) + 2) / 3), nil
}
