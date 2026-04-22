package types

import (
	"cosmossdk.io/errors"
	"github.com/cometbft/cometbft/crypto/secp256k1"
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

	if len(k.CosmosKey) != secp256k1.PubKeySize {
		return errors.Wrap(ErrInvalidPublicKey, "cosmos public key must be secp256k1 (33 bytes)")
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
