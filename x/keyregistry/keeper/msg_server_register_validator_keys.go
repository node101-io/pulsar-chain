package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// handleValidatorRegistration encapsulates the validator registration flow
// where the Cosmos-side key is the validator public key.
func (k msgServer) handleValidatorRegistration(ctx context.Context, msg *types.MsgRegisterKeys) error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errorsmod.Wrap(types.ErrInvalidCreatorAddress, "")
	}

	err = types.ValidatorPublicKeyPair{
		MinaKey:   msg.MinaPublicKey,
		CosmosKey: msg.CosmosPublicKey,
	}.Validate()
	if err != nil {
		return errorsmod.Wrap(types.ErrInvalidPublicKey, "pubkeys must be valid")
	}

	return k.persistValidatorRegistration(ctx, msg)
}

func (k msgServer) persistValidatorRegistration(ctx context.Context, msg *types.MsgRegisterKeys) error {
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

	minaSigValidity := VerifyValidatorMinaSig(msg.MinaSignature, msg.CosmosPublicKey, msg.MinaPublicKey)
	cosmosSigValidity := VerifyValidatorCosmosSig(msg.CosmosSignature, msg.MinaPublicKey, msg.CosmosPublicKey)
	if !minaSigValidity || !cosmosSigValidity {
		return errorsmod.Wrap(types.ErrInvalidSignature, "invalid cosmos or mina signature")
	}

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
