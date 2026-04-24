package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func (k msgServer) handleUserRegistration(ctx context.Context, msg *types.MsgRegisterKeys) error {
	_, err := k.addressCodec.StringToBytes(msg.Creator)
	if err != nil {
		return errorsmod.Wrap(types.ErrInvalidCreatorAddress, "")
	}

	err = types.UserPublicKeyPair{
		MinaKey:   msg.MinaPublicKey,
		CosmosKey: msg.CosmosPublicKey,
	}.Validate()
	if err != nil {
		return errorsmod.Wrap(types.ErrInvalidPublicKey, "")
	}

	cosmosAddr, err := deriveAddressFromPubkey(msg.ActorType, msg.CosmosPublicKey)
	if err != nil {
		return err
	}

	if msg.Creator != cosmosAddr {
		return errorsmod.Wrap(types.ErrInvalidSigner, "")
	}

	return k.persistUserRegistration(ctx, msg)
}

func (k msgServer) persistUserRegistration(ctx context.Context, msg *types.MsgRegisterKeys) error {
	cosmosKeyExists, err := k.Keeper.userCosmosToMina.Has(ctx, msg.CosmosPublicKey)
	if err != nil {
		return err
	}
	minaKeyExists, err := k.Keeper.userMinaToCosmos.Has(ctx, msg.MinaPublicKey)
	if err != nil {
		return err
	}
	if cosmosKeyExists || minaKeyExists {
		return errorsmod.Wrap(types.ErrUserSecondaryKeyExists, "")
	}

	minaSigValidity := VerifyUserMinaSig(msg.MinaSignature, msg.CosmosPublicKey, msg.MinaPublicKey)
	cosmosSigValidity := VerifyUserCosmosSig(msg.CosmosSignature, msg.MinaPublicKey, msg.CosmosPublicKey)
	if !minaSigValidity || !cosmosSigValidity {
		return errorsmod.Wrap(types.ErrInvalidSignature, "invalid cosmos or mina signature")
	}

	err = k.Keeper.userCosmosToMina.Set(ctx, msg.CosmosPublicKey, msg.MinaPublicKey)
	if err != nil {
		return err
	}
	err = k.Keeper.userMinaToCosmos.Set(ctx, msg.MinaPublicKey, msg.CosmosPublicKey)
	if err != nil {
		return err
	}

	return nil
}
