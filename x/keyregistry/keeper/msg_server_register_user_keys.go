package keeper

import (
	"context"

	"cosmossdk.io/errors"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func (k msgServer) handleUserRegistration(ctx context.Context, msg *types.MsgRegisterKeys) error {
	_, err := k.addressCodec.StringToBytes(msg.Creator)
	if err != nil {
		return errors.Wrap(types.ErrInvalidCreatorAddress, "creator address must be a valid bech32 address")
	}

	err = types.UserPublicKeyPair{
		MinaKey:   msg.MinaPublicKey,
		CosmosKey: msg.CosmosPublicKey,
	}.Validate()
	if err != nil {
		return err
	}

	cosmosAddr, err := deriveAddressFromPubkey(msg.ActorType, msg.CosmosPublicKey)
	if err != nil {
		return err
	}

	if msg.Creator != cosmosAddr {
		return errors.Wrap(types.ErrInvalidCreatorAddress, "creator address does not match the provided cosmos public key")
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
		return errors.Wrap(types.ErrUserSecondaryKeyExists, "provided cosmos or mina public key is already registered")
	}

	minaSigValidity, err := VerifyMinaSig(msg.MinaSignature, msg.CosmosPublicKey, msg.MinaPublicKey, msg.ActorType)
	if err != nil {
		return err
	}
	cosmosSigValidity := VerifyUserCosmosSig(msg.CosmosSignature, msg.MinaPublicKey, msg.CosmosPublicKey)
	if !minaSigValidity || !cosmosSigValidity {
		return errors.Wrap(types.ErrInvalidSignature, "invalid cosmos or mina signature")
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
