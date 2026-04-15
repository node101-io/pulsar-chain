package keeper

import (
	"context"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// TODO: Implement Mina signature verification for users
func VerifyUserMinaSig(sig string, msg, minaAddress []byte) bool {
	return true
}

// TODO: Implement Cosmos signature verification for users
func VerifyUserCosmosSig(sig string, msg, cosmosAddress []byte) bool {
	return true
}

// TODO: Implement Mina signature verification for users
func VerifyValidatorMinaSig(sig string, msg, minaAddress []byte) bool {
	return true
}

// TODO: Implement Cosmos signature verification for users
func VerifyValidatorCosmosSig(sig string, msg, cosmosAddress []byte) bool {
	return true
}

// deriveAddressFromPubkey derives the expected signer address from the provided
// key material. Users provide secp256k1 account public keys, while validators
// provide consensus public keys.
func deriveAddressFromPubkey(cosmosAddress []byte, isUser bool) string {
	if !isUser {
		pubKey := ed25519.PubKey{
			Key: cosmosAddress,
		}
		validatorAddr := sdk.ConsAddress(pubKey.Address())
		return validatorAddr.String()
	}

	pubKey := secp256k1.PubKey{
		Key: cosmosAddress,
	}

	addr := sdk.AccAddress(pubKey.Address())
	return addr.String()
}

// RegisterKeys registers a Mina and Cosmos public key pair on chain.
// It verifies that:
//   - the creator address is valid
//   - the cosmos public key is a valid compressed secp256k1 key (33 bytes)
//   - the creator address matches the provided cosmos public key
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
	}
	if err != nil {
		return nil, err
	}

	return &types.MsgRegisterKeysResponse{}, nil
}
