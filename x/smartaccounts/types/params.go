package types

import (
	"bytes"
	"fmt"
)

const VerificationKeyHashSize = 32

// NewParams creates a new Params instance.
func NewParams(verificationKeyHash []byte) Params {
	return Params{VerificationKeyHash: verificationKeyHash}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return Params{
		VerificationKeyHash: bytes.Repeat([]byte{0x00}, VerificationKeyHashSize),
	}
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if len(p.VerificationKeyHash) != VerificationKeyHashSize {
		return fmt.Errorf(
			"verification key hash must be %d bytes: got %d",
			VerificationKeyHashSize,
			len(p.VerificationKeyHash),
		)
	}

	return nil
}
