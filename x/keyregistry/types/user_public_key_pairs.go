package types

import (
	"cosmossdk.io/errors"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/node101-io/mina-signer-go/keys"
)

func NewUserPublicKeyPairs() []*UserPublicKeyPair {
	return []*UserPublicKeyPair{}
}

func DefaultUserPublicKeyPair() []*UserPublicKeyPair {
	return NewUserPublicKeyPairs()
}

func ValidateUserPublicKeyPair(k UserPublicKeyPair) error {

	if len(k.CosmosKey) != secp256k1.PubKeySize {
		return errors.Wrap(ErrInvalidPublicKey, "cosmos public key must be secp256k1 (33 bytes)")
	}

	if len(k.MinaKey) != keys.PublicKeyTotalByteSize {
		return errors.Wrap(ErrInvalidPublicKey, "mina public key must be compressed (33 bytes)")
	}
	return nil
}

// Validate validates the set of params.
func (k UserPublicKeyPair) Validate() error {
	return ValidateUserPublicKeyPair(k)
}
