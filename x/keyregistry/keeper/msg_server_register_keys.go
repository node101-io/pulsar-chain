package keeper

import (
	"context"

	"cosmossdk.io/errors"
	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minafield "github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/publickey"
	"github.com/node101-io/mina-signer-go/signature"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// RegistrationNetworkID is the Mina signature domain registration signatures
// are verified under. It must match the network users' wallets are connected
// to: a wallet derives the domain from its own network and offers no way for
// a dApp to override it, so a domain derived from anything else — the actor
// type, say — is one no wallet can produce, locking every wallet-based client
// out of registration.
//
// Mina devnet and testnet share this domain, so one constant covers both.
// Pairing this chain with Mina MAINNET means changing it to mina.MainNet —
// app.go refuses to start if this and app.toml's mina.network_id disagree,
// so the two cannot silently drift.
const RegistrationNetworkID = mina.TestNet

// VerifyMinaSig checks a Mina signature over the registration challenge
// against `minaAddress`. Domain separation between actors lives in the
// challenge itself; see types.RegistrationChallenge.
func VerifyMinaSig(sig []byte, challenge *minafield.FieldElement, minaAddress []byte) (bool, error) {

	minaPk, err := publickey.NewPublicKeyFromBytes(minaAddress, RegistrationNetworkID)
	if err != nil {
		return false, errors.Wrapf(types.ErrInvalidPublicKey, "invalid mina public key: %v", err)
	}

	minaSig, err := signature.NewSignatureFromBytes(sig)
	if err != nil {
		return false, errors.Wrapf(types.ErrInvalidSignature, "invalid mina signature: %v", err)
	}

	valid, err := minaPk.VerifyField(minaSig, challenge)
	if err != nil {
		return false, errors.Wrapf(types.ErrInvalidSignature, "failed to verify mina signature: %v", err)
	}

	return valid, nil
}

func VerifyUserCosmosSig(sig []byte, msg, cosmosAddress []byte) bool {

	cosmosPk := secp256k1.PubKey{
		Key: cosmosAddress,
	}

	return cosmosPk.VerifySignature(msg, sig)
}

func VerifyValidatorCosmosSig(sig []byte, msg, cosmosAddress []byte) bool {

	var cosmosValidatorPubKey ed25519.PubKey = cosmosAddress

	return cosmosValidatorPubKey.VerifySignature(msg, sig)
}

// deriveAddressFromPubkey derives the expected signer address from the provided
// key material.
func deriveAddressFromPubkey(actorType types.ActorType, cosmosPublicKey []byte) (string, error) {

	if actorType != types.ActorType_USER {
		return "", types.ErrInvalidActorType
	}

	pubKey := secp256k1.PubKey{
		Key: cosmosPublicKey,
	}

	addr := sdk.AccAddress(pubKey.Address())
	return addr.String(), nil

}

// RegisterKeys registers a Mina and Cosmos public key pair on chain.
// It verifies that:
//   - the creator address is valid
//   - the provided public keys match the actor-specific key formats
//   - the creator address matches the provided cosmos public key for user registrations
//   - neither the cosmos nor mina public key is already registered
//   - both the mina and cosmos signatures are valid
//
// If all checks pass, the key pair is stored in both the CosmosToMina and MinaToCosmos maps.
func (k msgServer) RegisterKeys(ctx context.Context, msg *types.MsgRegisterKeys) (*types.MsgRegisterKeysResponse, error) {

	var err error

	switch msg.ActorType {
	case types.ActorType_USER:
		err = k.handleUserRegistration(ctx, msg)
	case types.ActorType_VALIDATOR:
		err = k.handleValidatorRegistration(ctx, msg)
	default:
		return nil, types.ErrInvalidActorType
	}
	if err != nil {
		return nil, err
	}

	return &types.MsgRegisterKeysResponse{}, nil
}
