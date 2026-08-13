package keeper

import (
	"bytes"
	"context"

	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func (k msgServer) UpdateKeys(ctx context.Context, msg *types.MsgUpdateKeys) (*types.MsgUpdateKeysResponse, error) {
	var err error

	if err := types.ValidateMinaPublicKey(msg.PrevMinaPublicKey); err != nil {
		return nil, errors.Wrap(err, "previous mina public key")
	}
	if err := types.ValidateMinaPublicKey(msg.NewMinaPublicKey); err != nil {
		return nil, errors.Wrap(err, "new mina public key")
	}

	switch msg.ActorType {
	case types.ActorType_USER:
		err = k.updateUserKeys(ctx, msg)
	case types.ActorType_VALIDATOR:
		err = k.updateValidatorKeys(ctx, msg)
	default:
		return nil, types.ErrInvalidActorType
	}
	if err != nil {
		return nil, err
	}

	return &types.MsgUpdateKeysResponse{}, nil
}

func (k msgServer) updateUserKeys(ctx context.Context, msg *types.MsgUpdateKeys) error {
	_, err := k.addressCodec.StringToBytes(msg.Creator)
	if err != nil {
		return errors.Wrap(types.ErrInvalidCreatorAddress, "creator address must be a valid bech32 address")
	}

	exists, err := k.UserMinaToCosmosHas(ctx, msg.PrevMinaPublicKey)
	if err != nil {
		return err
	}
	if !exists {
		return types.ErrUserNotRegistered
	}

	cosmosPublicKey, err := k.UserGetMinaToCosmos(ctx, msg.PrevMinaPublicKey)
	if err != nil {
		return err
	}

	cosmosAddr, err := deriveAddressFromPubkey(msg.ActorType, cosmosPublicKey)
	if err != nil {
		return err
	}

	if msg.Creator != cosmosAddr {
		return errors.Wrap(types.ErrInvalidCreatorAddress, "creator address does not match the registered cosmos public key")
	}

	exists, err = k.UserCosmosToMinaHas(ctx, cosmosPublicKey)
	if err != nil {
		return err
	}
	if !exists {
		return types.ErrUserNotRegistered
	}

	currentMinaPublicKey, err := k.UserGetCosmosToMina(ctx, cosmosPublicKey)
	if err != nil {
		return err
	}
	if !bytes.Equal(currentMinaPublicKey, msg.PrevMinaPublicKey) {
		return types.ErrUserNotRegistered
	}

	exists, err = k.UserMinaToCosmosHas(ctx, msg.NewMinaPublicKey)
	if err != nil {
		return err
	}
	if exists && !bytes.Equal(msg.NewMinaPublicKey, msg.PrevMinaPublicKey) {
		return types.ErrUserSecondaryKeyExists
	}

	if !VerifyUserCosmosSig(msg.CosmosSignature, msg.NewMinaPublicKey, cosmosPublicKey) {
		return types.ErrInvalidSignature
	}

	challenge, err := types.RegistrationChallenge(msg.ActorType, cosmosPublicKey)
	if err != nil {
		return err
	}

	minaSigValidity, err := VerifyMinaSig(msg.NewMinaSignature, challenge, msg.NewMinaPublicKey)
	if err != nil {
		return err
	}

	if !minaSigValidity {
		return types.ErrInvalidSignature
	}

	err = k.userMinaToCosmos.Remove(ctx, msg.PrevMinaPublicKey)
	if err != nil {
		return err
	}
	err = k.userCosmosToMina.Set(ctx, cosmosPublicKey, msg.NewMinaPublicKey)
	if err != nil {
		return err
	}

	return k.userMinaToCosmos.Set(ctx, msg.NewMinaPublicKey, cosmosPublicKey)
}

func (k msgServer) updateValidatorKeys(ctx context.Context, msg *types.MsgUpdateKeys) error {
	if _, err := sdk.AccAddressFromBech32(msg.Creator); err != nil {
		return errors.Wrap(types.ErrInvalidCreatorAddress, "creator address must be a valid bech32 address")
	}

	exists, err := k.ValidatorMinaToCosmosHas(ctx, msg.PrevMinaPublicKey)
	if err != nil {
		return err
	}
	if !exists {
		return types.ErrValidatorNotRegistered
	}

	cosmosPublicKey, err := k.ValidatorGetMinaToCosmos(ctx, msg.PrevMinaPublicKey)
	if err != nil {
		return err
	}

	exists, err = k.ValidatorCosmosToMinaHas(ctx, cosmosPublicKey)
	if err != nil {
		return err
	}
	if !exists {
		return types.ErrValidatorNotRegistered
	}

	currentMinaPublicKey, err := k.ValidatorGetCosmosToMina(ctx, cosmosPublicKey)
	if err != nil {
		return err
	}
	if !bytes.Equal(currentMinaPublicKey, msg.PrevMinaPublicKey) {
		return types.ErrValidatorNotRegistered
	}

	exists, err = k.ValidatorMinaToCosmosHas(ctx, msg.NewMinaPublicKey)
	if err != nil {
		return err
	}
	if exists && !bytes.Equal(msg.NewMinaPublicKey, msg.PrevMinaPublicKey) {
		return types.ErrValidatorSecondaryKeyExists
	}

	if !VerifyValidatorCosmosSig(msg.CosmosSignature, msg.NewMinaPublicKey, cosmosPublicKey) {
		return types.ErrInvalidSignature
	}

	challenge, err := types.RegistrationChallenge(msg.ActorType, cosmosPublicKey)
	if err != nil {
		return err
	}

	minaSigValidity, err := VerifyMinaSig(msg.NewMinaSignature, challenge, msg.NewMinaPublicKey)
	if err != nil {
		return err
	}

	if !minaSigValidity {
		return types.ErrInvalidSignature
	}

	err = k.validatorMinaToCosmos.Remove(ctx, msg.PrevMinaPublicKey)
	if err != nil {
		return err
	}
	err = k.validatorCosmosToMina.Set(ctx, cosmosPublicKey, msg.NewMinaPublicKey)
	if err != nil {
		return err
	}

	return k.validatorMinaToCosmos.Set(ctx, msg.NewMinaPublicKey, cosmosPublicKey)
}
