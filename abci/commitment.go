package abci

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	verificationTypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

func MockVerifier(proofHashes [][]byte) ([]bool, error) {
	results := make([]bool, len(proofHashes))

	for i := range results {
		results[i] = true
	}

	return results, nil
}

func generateSecretSalt() []byte {
	salt := make([]byte, secretSaltLength)

	// Rand func never returns an error.
	_, _ = rand.Read(salt)

	return salt
}

func validateProofCommitment(commitment []byte) bool {
	return len(commitment) == proofCommitmentLength
}

func (h *ABCIHandler) GenerateCommitmentForVerifiedProofs(ctx sdk.Context) ([]byte, error) {

	var verifiedProofFirst, verifiedProofSecond []byte
	var proofHashCountFirstBlock, proofHashCountSecondBlock int

	var firstBlockProofs []verificationTypes.ProofCommitment

	currentBlockHeight := ctx.BlockHeight()

	firstBlockProofHashes, firstBlockProofIndexes, err := h.verificationKeeper.GetProofHashesByBlockHeight(ctx, currentBlockHeight-3)
	if err != nil {
		return nil, err
	}
	if len(firstBlockProofHashes) != len(firstBlockProofIndexes) {
		return nil, fmt.Errorf("first block proof hash/index count mismatch")
	}

	secondBlockProofHashes, secondBlockProofIndexes, err := h.verificationKeeper.GetProofHashesByBlockHeight(ctx, currentBlockHeight-2)
	if err != nil {
		return nil, err
	}
	if len(secondBlockProofHashes) != len(secondBlockProofIndexes) {
		return nil, fmt.Errorf("second block proof hash/index count mismatch")
	}

	var proofHashes [][]byte

	proofHashes = append(proofHashes, firstBlockProofHashes...)
	proofHashes = append(proofHashes, secondBlockProofHashes...)

	var verifiedProofs []bool

	if len(proofHashes) > 0 {
		verifiedProofs, err = MockVerifier(proofHashes)
		if err != nil {
			return nil, err
		}
	}

	if len(verifiedProofs) != len(proofHashes) {
		return nil, fmt.Errorf(
			"verifier returned %d results for %d proofs",
			len(verifiedProofs),
			len(proofHashes),
		)
	}

	numberOfProofHashesFirstBlock := len(firstBlockProofHashes)

	for i := range proofHashes {

		isFirstBlock := i < numberOfProofHashesFirstBlock

		if isFirstBlock {
			commitment := verificationTypes.ProofCommitment{
				ProofIndex:   firstBlockProofIndexes[i],
				IsProofValid: verifiedProofs[i],
			}

			firstBlockProofs = append(firstBlockProofs, commitment)

			marshalled, err := commitment.Marshal()
			if err != nil {
				return nil, err
			}

			verifiedProofFirst = append(verifiedProofFirst, marshalled...)
			proofHashCountFirstBlock++

		} else {
			commitment := verificationTypes.ProofCommitment{
				ProofIndex:   secondBlockProofIndexes[i-numberOfProofHashesFirstBlock],
				IsProofValid: verifiedProofs[i],
			}

			marshalled, err := commitment.Marshal()
			if err != nil {
				return nil, err
			}

			verifiedProofSecond = append(verifiedProofSecond, marshalled...)
			proofHashCountSecondBlock++
		}
	}

	secretSaltFirst := generateSecretSalt()
	secretSaltSecond := generateSecretSalt()

	firstInput := append([]byte{}, secretSaltFirst...)
	firstInput = append(firstInput, encodeLength(proofHashCountFirstBlock)...)
	firstInput = append(firstInput, verifiedProofFirst...)

	firstHash := sha256.Sum256(firstInput)
	first128 := firstHash[:16]

	secondInput := append([]byte{}, secretSaltSecond...)
	secondInput = append(secondInput, encodeLength(proofHashCountSecondBlock)...)
	secondInput = append(secondInput, verifiedProofSecond...)

	secondHash := sha256.Sum256(secondInput)
	second128 := secondHash[:16]

	finalInput := append([]byte{}, first128...)
	finalInput = append(finalInput, second128...)

	finalHash := sha256.Sum256(finalInput)
	truncate := truncate128(finalHash)

	h.revealStore.Set(ctx.BlockHeight(), verificationTypes.ProofCommitmentReveal{
		FirstSecretSalt:  secretSaltFirst,
		FirstBlockProofs: firstBlockProofs,
		SecondLeafHash:   second128,
	})

	return truncate[:], nil
}

func truncate128(b [32]byte) [16]byte {
	var out [16]byte
	copy(out[:], b[:16])
	return out
}

func encodeLength(n int) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(n))
	return b[:]
}
