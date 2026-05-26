package types

import (
	"crypto/ed25519"

	"cosmossdk.io/errors"
	"github.com/node101-io/mina-signer-go/publickey"
)

func NewValidatorPublicKeyPairs() []*ValidatorPublicKeyPair {
	return []*ValidatorPublicKeyPair{}
}

func DefaultValidatorPublicKeyPair() []*ValidatorPublicKeyPair {
	return NewValidatorPublicKeyPairs()
}

// Validate validates the set of params.
// TODO: make strict validate once mina-signer-go is complete
func (k ValidatorPublicKeyPair) Validate() error {
	if len(k.CosmosKey) != ed25519.PublicKeySize {
		return errors.Wrap(ErrInvalidPublicKey, "cosmos public key must be ed25519 (32 bytes)")
	}

	if len(k.MinaKey) != publickey.Size() {
		return errors.Wrap(ErrInvalidPublicKey, "mina public key must be compressed (32 bytes)")
	}
	return nil
}
