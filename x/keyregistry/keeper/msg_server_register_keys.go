package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// TODO: Implement Mina signature verification for users
func VerifyUserMinaSig(sig string, msg, minaPublicKey []byte) bool {
	return true
}

// TODO: Implement Cosmos signature verification for users
func VerifyUserCosmosSig(sig string, msg, cosmosPublicKey []byte) bool {
	return true
}

// TODO: Implement Mina signature verification for users
func VerifyValidatorMinaSig(sig string, msg, minaPublicKey []byte) bool {
	return true
}

// TODO: Implement Cosmos signature verification for users
func VerifyValidatorCosmosSig(sig string, msg, cosmosPublicKey []byte) bool {
	return true
}

// deriveAddressFromPubkey derives a bech32 cosmos address from a compressed secp256k1 public key.
func deriveAddressFromPubkey(cosmosPublicKey []byte, isUser bool) string {
	pubKey := secp256k1.PubKey{
		Key: cosmosPublicKey,
	}
	if !isUser {
		validatorAddr := sdk.ConsAddress(pubKey.Address())
		return validatorAddr.String()
	}
	addr := sdk.AccAddress(pubKey.Address())
	return addr.String()
}

func ValidatorKeyRegister(k msgServer, ctx context.Context, msg *types.MsgRegisterKeys) error {
	// Check if either key is already registered to prevent duplicate registrations.
	cosmosKeyExists, err := k.Keeper.validatorCosmosToMina.Has(ctx, msg.CosmosPublicKey)
	if err != nil {
		return err
	}
	minaKeyExists, err := k.Keeper.validatorMinaToCosmos.Has(ctx, msg.MinaPublicKey)
	if err != nil {
		return err
	}
	if cosmosKeyExists || minaKeyExists {
		return errorsmod.Wrap(types.ErrValidatorSecondaryKeyExists, "")
	}

	// Verify that the mina key signed the cosmos public key and vice versa.
	// This proves ownership of both keys.
	minaSigValidity := VerifyValidatorMinaSig(msg.MinaSignature, msg.CosmosPublicKey, msg.MinaPublicKey)
	cosmosSigValidity := VerifyValidatorCosmosSig(msg.CosmosSignature, msg.MinaPublicKey, msg.CosmosPublicKey)

	if !minaSigValidity || !cosmosSigValidity {
		return errorsmod.Wrap(types.ErrInvalidSignature, "invalid cosmos or mina signature")
	}

	// Store the key pair in both directions to allow lookups by either key.
	err = k.Keeper.validatorCosmosToMina.Set(ctx, msg.CosmosPublicKey, msg.MinaPublicKey)
	if err != nil {
		return err
	}
	err = k.Keeper.validatorMinaToCosmos.Set(ctx, msg.MinaPublicKey, msg.CosmosPublicKey)
	if err != nil {
		return err
	}
	return nil
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

	if msg.IsUser {
		_, err := k.addressCodec.StringToBytes(msg.Creator)
		if err != nil {
			return nil, errorsmod.Wrap(types.ErrInvalidCreatorAddres, "")
		}
	} else {
		_, err := sdk.ConsAddressFromBech32(msg.Creator)
		if err != nil {
			return nil, errorsmod.Wrap(types.ErrInvalidCreatorAddres, "")
		}
	}

	// Ensure the cosmos and mina public keys are valid.
	err := types.ValidateKeyPair(types.KeyPair{
		MinaKey:   msg.MinaPublicKey,
		CosmosKey: msg.CosmosPublicKey,
	})
	if err != nil {
		return nil, errorsmod.Wrap(types.ErrInvalidPublicKey, "pubkeys must be valid")
	}

	// Ensure the creator address matches the provided cosmos public key
	// to prevent someone from registering a key pair on behalf of another address.
	derivedAddress := deriveAddressFromPubkey(msg.CosmosPublicKey, msg.IsUser)

	if derivedAddress != msg.Creator {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "creator does not match provided cosmos public key")
	}

	if !msg.IsUser {
		err := ValidatorKeyRegister(k, ctx, msg)
		if err != nil {
			return nil, err
		}
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
		return nil, errorsmod.Wrap(types.ErrUserSecondaryKeyExists, "")
	}

	// Verify that the mina key signed the cosmos public key and vice versa.
	// This proves ownership of both keys.
	minaSigValidity := VerifyUserMinaSig(msg.MinaSignature, msg.CosmosPublicKey, msg.MinaPublicKey)
	cosmosSigValidity := VerifyUserCosmosSig(msg.CosmosSignature, msg.MinaPublicKey, msg.CosmosPublicKey)

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
