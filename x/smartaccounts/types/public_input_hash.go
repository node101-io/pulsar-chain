package types

import (
	"crypto/sha256"
	"encoding/binary"
)

const publicInputFieldSize = 32

// ComputePublicInputsHash encodes and hashes the smart-account public inputs.
func ComputePublicInputsHash(inputs *PublicKeyInputs) ([]byte, error) {
	if inputs == nil {
		return nil, ErrNilPublicKeyInputs
	}
	if inputs.SessionPublicKey == nil {
		return nil, ErrNilPublicKey
	}
	if len(inputs.SessionPublicKey) != SessionPublicKeySize {
		return nil, ErrPublicKeyInvalidLength
	}
	if inputs.ExpiresAtHeight == 0 {
		return nil, ErrInvalidExpirationHeight
	}
	if len(inputs.Identity) == 0 {
		return nil, ErrNilIdentity
	}
	if len(inputs.Identity) != IdentitySize {
		return nil, ErrIdentityInvalidLength
	}

	rawPublicInputs := make([]byte, 0, SessionPublicKeySize+publicInputFieldSize+IdentitySize)
	rawPublicInputs = append(rawPublicInputs, inputs.SessionPublicKey...)

	var expiresAtHeight [publicInputFieldSize]byte
	binary.BigEndian.PutUint64(
		expiresAtHeight[publicInputFieldSize-8:],
		inputs.ExpiresAtHeight,
	)
	rawPublicInputs = append(rawPublicInputs, expiresAtHeight[:]...)
	rawPublicInputs = append(rawPublicInputs, inputs.Identity...)

	hash := sha256.Sum256(rawPublicInputs)
	return hash[:], nil
}
