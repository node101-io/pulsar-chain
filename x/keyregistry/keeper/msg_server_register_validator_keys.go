package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// handleValidatorRegistration encapsulates the validator registration flow
// where the Cosmos-side key is the validator consensus public key.
func (k msgServer) handleValidatorRegistration(ctx context.Context, msg *types.MsgRegisterKeys) error {
	_, err := sdk.ConsAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrap(types.ErrInvalidCreatorAddres, "")
	}

	err = types.ValidatePublicKeyPair(types.PublicKeyPair{
		MinaKey:   msg.MinaAddress,
		CosmosKey: msg.CosmosAddress,
	})
	if err != nil {
		return errorsmod.Wrap(types.ErrInvalidPublicKey, "pubkeys must be valid")
	}

	derivedAddress := deriveAddressFromPubkey(msg.CosmosAddress, false)
	if derivedAddress != msg.Creator {
		return errorsmod.Wrap(types.ErrInvalidSigner, "creator does not match provided cosmos consensus public key")
	}

	return k.persistValidatorRegistration(ctx, msg)
}

func (k msgServer) persistValidatorRegistration(ctx context.Context, msg *types.MsgRegisterKeys) error {
	cosmosKeyExists, err := k.Keeper.validatorCosmosToMina.Has(ctx, msg.CosmosAddress)
	if err != nil {
		return err
	}
	minaKeyExists, err := k.Keeper.validatorMinaToCosmos.Has(ctx, msg.MinaAddress)
	if err != nil {
		return err
	}
	if cosmosKeyExists || minaKeyExists {
		return errorsmod.Wrap(types.ErrValidatorSecondaryKeyExists, "")
	}

	minaSigValidity := VerifyValidatorMinaSig(msg.MinaSignature, msg.CosmosAddress, msg.MinaAddress)
	cosmosSigValidity := VerifyValidatorCosmosSig(msg.CosmosSignature, msg.MinaAddress, msg.CosmosAddress)
	if !minaSigValidity || !cosmosSigValidity {
		return errorsmod.Wrap(types.ErrInvalidSignature, "invalid cosmos or mina signature")
	}

	err = k.Keeper.validatorCosmosToMina.Set(ctx, msg.CosmosAddress, msg.MinaAddress)
	if err != nil {
		return err
	}
	err = k.Keeper.validatorMinaToCosmos.Set(ctx, msg.MinaAddress, msg.CosmosAddress)
	if err != nil {
		return err
	}

	return nil
}
