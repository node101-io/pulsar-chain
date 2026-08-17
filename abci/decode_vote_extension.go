package abci

import (
	"fmt"

	verificationTypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

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

	if voteExtension.Reveal != nil {
		if err := validateReveal(*voteExtension.Reveal); err != nil {
			return VoteExtension{}, err
		}
	}

	return voteExtension, nil
}

func validateReveal(reveal verificationTypes.ProofCommitmentReveal) error {

	if len(reveal.SecondLeafHash) != 16 {
		return fmt.Errorf("")
	}

	if len(reveal.FirstSecretSalt) != 16 {
		return fmt.Errorf("")
	}

	return nil
}
