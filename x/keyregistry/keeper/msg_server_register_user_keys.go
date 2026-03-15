package keeper

import (
	"bytes"
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// handleUserRegistration encapsulates the user registration flow where both
// sides of the mapping are addresses, not public keys.
func (k msgServer) handleUserRegistration(ctx context.Context, msg *types.MsgRegisterKeys) error {
	creatorAddress, err := k.addressCodec.StringToBytes(msg.Creator)
	if err != nil {
		return errorsmod.Wrap(types.ErrInvalidCreatorAddres, "")
	}

	err = types.ValidateAddressPair(types.AddressPair{
		MinaAddr:   msg.MinaAddress,
		CosmosAddr: msg.CosmosAddress,
	})
	if err != nil {
		return errorsmod.Wrap(types.ErrInvalidAddress, "address must be valid")
	}

	if !bytes.Equal(creatorAddress, msg.CosmosAddress) {
		return errorsmod.Wrap(types.ErrInvalidSigner, "creator does not match provided cosmos address")
	}

	return k.persistUserRegistration(ctx, msg)
}

func (k msgServer) persistUserRegistration(ctx context.Context, msg *types.MsgRegisterKeys) error {
	cosmosKeyExists, err := k.Keeper.userCosmosToMina.Has(ctx, msg.CosmosAddress)
	if err != nil {
		return err
	}
	minaKeyExists, err := k.Keeper.userMinaToCosmos.Has(ctx, msg.MinaAddress)
	if err != nil {
		return err
	}
	if cosmosKeyExists || minaKeyExists {
		return errorsmod.Wrap(types.ErrUserSecondaryKeyExists, "")
	}

	minaSigValidity := VerifyUserMinaSig(msg.MinaSignature, msg.CosmosAddress, msg.MinaAddress)
	cosmosSigValidity := VerifyUserCosmosSig(msg.CosmosSignature, msg.MinaAddress, msg.CosmosAddress)
	if !minaSigValidity || !cosmosSigValidity {
		return errorsmod.Wrap(types.ErrInvalidSignature, "invalid cosmos or mina signature")
	}

	err = k.Keeper.userCosmosToMina.Set(ctx, msg.CosmosAddress, msg.MinaAddress)
	if err != nil {
		return err
	}
	err = k.Keeper.userMinaToCosmos.Set(ctx, msg.MinaAddress, msg.CosmosAddress)
	if err != nil {
		return err
	}

	return nil
}
