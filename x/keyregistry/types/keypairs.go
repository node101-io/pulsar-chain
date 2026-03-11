package types

import (
	"cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/node101-io/mina-signer-go/keys"
)

func NewKeyPairs() []*KeyPair {
	return []*KeyPair{}
}
func DefaultKeyPairs() []*KeyPair {
	return NewKeyPairs()
}

func ValidateKeyPair(k KeyPair) error {
	if len(k.CosmosKey) != secp256k1.PubKeySize {
		return errors.Wrap(ErrInvalidPublicKey, "cosmos public key must be compressed (33 bytes)")
	}

	if len(k.MinaKey) != keys.PublicKeyTotalByteSize {
		return errors.Wrap(ErrInvalidPublicKey, "mina public key must be compressed (33 bytes)")
	}
	return nil
}

// Validate validates the set of params.
func (k KeyPair) Validate() error {
	return ValidateKeyPair(k)
}
