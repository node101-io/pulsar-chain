package keeper

import (
	errorsmod "cosmossdk.io/errors"
	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minafield "github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/publickey"
	"github.com/node101-io/mina-signer-go/signature"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

const walletFieldSignatureNetworkID = mina.TestNet

func verifyMinaFieldSignature(sig []byte, challenge *minafield.FieldElement, minaPublicKey []byte) (bool, error) {
	publicKey, err := publickey.NewPublicKeyFromBytes(minaPublicKey, walletFieldSignatureNetworkID)
	if err != nil {
		return false, errorsmod.Wrapf(types.ErrInvalidPublicKey, "invalid mina public key: %v", err)
	}

	minaSignature, err := signature.NewSignatureFromBytes(sig)
	if err != nil {
		return false, errorsmod.Wrapf(types.ErrInvalidSignature, "invalid mina signature: %v", err)
	}

	valid, err := publicKey.VerifyField(minaSignature, challenge)
	if err != nil {
		return false, errorsmod.Wrapf(types.ErrInvalidSignature, "verify mina signature: %v", err)
	}

	return valid, nil
}

func verifyValidatorConsensusSignature(signature, challenge, consensusPublicKey []byte) bool {
	return ed25519.PubKey(consensusPublicKey).VerifySignature(challenge, signature)
}

func deriveUserAddress(cosmosPublicKey []byte) string {
	publicKey := secp256k1.PubKey{Key: cosmosPublicKey}
	return sdk.AccAddress(publicKey.Address()).String()
}
