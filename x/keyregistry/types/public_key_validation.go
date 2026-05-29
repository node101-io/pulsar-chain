package types

import (
	"crypto/ed25519"

	"cosmossdk.io/errors"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/node101-io/mina-signer-go/publickey"
)

func ValidateUserCosmosPublicKey(cosmosKey []byte) error {
	if len(cosmosKey) != secp256k1.PubKeySize {
		return errors.Wrapf(ErrInvalidPublicKey, "cosmos public key must be secp256k1 (%d bytes)", secp256k1.PubKeySize)
	}

	return nil
}

func ValidateValidatorCosmosPublicKey(cosmosKey []byte) error {
	if len(cosmosKey) != ed25519.PublicKeySize {
		return errors.Wrapf(ErrInvalidPublicKey, "cosmos public key must be ed25519 (%d bytes)", ed25519.PublicKeySize)
	}

	return nil
}

func ValidateMinaPublicKey(minaKey []byte) error {
	if err := publickey.Validate(minaKey); err != nil {
		return errors.Wrapf(ErrInvalidPublicKey, "invalid mina public key: %v", err)
	}

	return nil
}
