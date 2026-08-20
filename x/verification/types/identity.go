package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
)

const verificationIDDomain = "pulsar/verification/v1\x00"

// IsSupportedProofType reports whether every validator is expected to support
// the verifier family. New values require a coordinated chain upgrade.
func IsSupportedProofType(proofType ProofType) bool {
	switch proofType {
	case ProofType_PROOF_TYPE_MINA_PICKLES,
		ProofType_PROOF_TYPE_NOIR_BARRETENBERG:
		return true
	default:
		return false
	}
}

// ComputeVerificationID binds the proof, statement, key, and verifier family
// into the permanent identifier used by consensus and the sidecar hot path.
func ComputeVerificationID(
	proofType ProofType,
	proofHash []byte,
	publicInputsHash []byte,
	verificationKeyHash []byte,
) ([VerificationIDSize]byte, error) {
	var id [VerificationIDSize]byte
	if !IsSupportedProofType(proofType) {
		return id, ErrInvalidProofType
	}
	if len(proofHash) != ProofHashSize {
		return id, ErrInvalidProofHash
	}
	if len(publicInputsHash) != PublicInputsHashSize {
		return id, ErrInvalidPublicInputsHash
	}
	if len(verificationKeyHash) != VerificationKeyHashSize {
		return id, ErrInvalidVerificationKeyHash
	}

	hasher := sha256.New()
	_, _ = hasher.Write([]byte(verificationIDDomain))
	var encodedType [4]byte
	binary.BigEndian.PutUint32(encodedType[:], uint32(proofType))
	_, _ = hasher.Write(encodedType[:])
	_, _ = hasher.Write(proofHash)
	_, _ = hasher.Write(publicInputsHash)
	_, _ = hasher.Write(verificationKeyHash)
	copy(id[:], hasher.Sum(nil))

	return id, nil
}

// ValidateProofRecord rejects malformed or internally inconsistent consensus
// metadata before it can be used for lookup, commitment construction, or query.
func ValidateProofRecord(record ProofRecord) error {
	if len(record.VerificationId) != VerificationIDSize {
		return ErrInvalidVerificationID
	}
	id, err := ComputeVerificationID(
		record.ProofType,
		record.ProofHash,
		record.PublicInputsHash,
		record.VerificationKeyHash,
	)
	if err != nil {
		return err
	}
	if !bytes.Equal(record.VerificationId, id[:]) {
		return ErrInvalidVerificationID
	}

	return nil
}
