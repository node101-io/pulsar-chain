package types

import (
	"cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/node101-io/mina-signer-go/keys"
)

func NewPublicKeyPairs() []*PublicKeyPair {
	return []*PublicKeyPair{}
}
func DefaultUserPublicKeyPairs() []*PublicKeyPair {
	return NewPublicKeyPairs()
}

func DefaultPublicKeyPair() []*PublicKeyPair {
	return NewPublicKeyPairs()
}

func ValidatePublicKeyPair(k PublicKeyPair) error {
	if len(k.CosmosKey) != ed25519.PubKeySize {
		return errors.Wrap(ErrInvalidPublicKey, "cosmos consensus public key must be ed25519 (32 bytes)")
	}

	if len(k.MinaKey) != keys.PublicKeyTotalByteSize {
		return errors.Wrap(ErrInvalidPublicKey, "mina public key must be compressed (33 bytes)")
	}
	return nil
}

// Validate validates the set of params.
func (k PublicKeyPair) Validate() error {
	return ValidatePublicKeyPair(k)
}
