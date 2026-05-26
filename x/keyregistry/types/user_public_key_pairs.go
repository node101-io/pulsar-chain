package types

import (
	"cosmossdk.io/errors"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/node101-io/mina-signer-go/publickey"
)

func NewUserPublicKeyPairs() []*UserPublicKeyPair {
	return []*UserPublicKeyPair{}
}

func DefaultUserPublicKeyPair() []*UserPublicKeyPair {
	return NewUserPublicKeyPairs()
}

// Validate validates the set of params.
// TODO: make strict validate once mina-signer-go is complete
func (k UserPublicKeyPair) Validate() error {
	if len(k.CosmosKey) != secp256k1.PubKeySize {
		return errors.Wrap(ErrInvalidPublicKey, "cosmos public key must be secp256k1 (33 bytes)")
	}

	if len(k.MinaKey) != publickey.Size() {
		return errors.Wrap(ErrInvalidPublicKey, "mina public key must be compressed (32 bytes)")
	}
	return nil
}
