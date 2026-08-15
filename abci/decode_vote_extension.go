package abci

import "fmt"

func decodeVoteExtension(bz []byte) (VoteExtension, error) {
	var voteExtension VoteExtension
	if err := voteExtension.Unmarshal(bz); err != nil {
		return VoteExtension{}, fmt.Errorf("%w: %v", ErrInvalidVoteExtensionEncoding, err)
	}

	if len(voteExtension.Signature) == 0 {
		return VoteExtension{}, fmt.Errorf("%w: missing vote extension signature", ErrInvalidVoteExtensionEncoding)
	}

	if !validateProofCommitment(voteExtension.ProofCommitment) {
		return VoteExtension{}, fmt.Errorf(
			"%w: expected %d bytes, got %d",
			ErrInvalidProofCommitment,
			proofCommitmentLength,
			len(voteExtension.ProofCommitment),
		)
	}

	return voteExtension, nil
}
