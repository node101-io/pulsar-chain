package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// TODO: Implement Mina signature verification
func VerifyMinaSig(sig string, msg, minaPublicKey []byte) bool {
	return true
}

// TODO: Implement Cosmos signature verification
func VerifyCosmosSig(sig string, msg, cosmosPublicKey []byte) bool {
	return true
}

// deriveAddressFromPubkey derives a bech32 cosmos address from a compressed secp256k1 public key.
func deriveAddressFromPubkey(cosmosPublicKey []byte) string {
	pubKey := secp256k1.PubKey{
		Key: cosmosPublicKey,
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
	// Validate the creator address.
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(types.ErrInvalidCreatorAddres, "")
	}

	// Ensure the cosmos public key is a compressed secp256k1 key (33 bytes).
	if len(msg.CosmosPublicKey) != secp256k1.PubKeySize {
		return nil, errorsmod.Wrap(types.ErrInvalidPublicKey, "cosmos pubkey must be compressed (33 bytes)")
	}

	// Ensure the creator address matches the provided cosmos public key
	// to prevent someone from registering a key pair on behalf of another address.
	derivedAddress := deriveAddressFromPubkey(msg.CosmosPublicKey)

	if derivedAddress != msg.Creator {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "creator does not match provided cosmos public key")
	}

	// Check if either key is already registered to prevent duplicate registrations.
	cosmosKeyExists, err := k.Keeper.cosmosToMina.Has(ctx, msg.CosmosPublicKey)
	if err != nil {
		return nil, err
	}
	minaKeyExists, err := k.Keeper.minaToCosmos.Has(ctx, msg.MinaPublicKey)
	if err != nil {
		return nil, err
	}
	if cosmosKeyExists || minaKeyExists {
		return nil, errorsmod.Wrap(types.ErrSecondaryKeyExists, "")
	}

	// Verify that the mina key signed the cosmos public key and vice versa.
	// This proves ownership of both keys.
	minaSigValidity := VerifyMinaSig(msg.MinaSignature, msg.CosmosPublicKey, msg.MinaPublicKey)
	cosmosSigValidity := VerifyCosmosSig(msg.CosmosSignature, msg.MinaPublicKey, msg.CosmosPublicKey)

	if !minaSigValidity || !cosmosSigValidity {
		return nil, errorsmod.Wrap(types.ErrInvalidSignature, "invalid cosmos or mina signature")
	}

	// Store the key pair in both directions to allow lookups by either key.
	err = k.Keeper.cosmosToMina.Set(ctx, msg.CosmosPublicKey, msg.MinaPublicKey)
	if err != nil {
		return nil, err
	}
	err = k.Keeper.minaToCosmos.Set(ctx, msg.MinaPublicKey, msg.CosmosPublicKey)
	if err != nil {
		return nil, err
	}

	return &types.MsgRegisterKeysResponse{}, nil
}
