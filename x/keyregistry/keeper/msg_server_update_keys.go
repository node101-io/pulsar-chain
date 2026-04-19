package keeper

import (
	"bytes"
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func (k msgServer) UpdateKeys(ctx context.Context, msg *types.MsgUpdateKeys) (*types.MsgUpdateKeysResponse, error) {
	var err error

	switch msg.ActorType {
	case types.ActorType_USER:
		err = k.updateUserKeys(ctx, msg)
	case types.ActorType_VALIDATOR:
		err = k.updateValidatorKeys(ctx, msg)
	}
	if err != nil {
		return nil, err
	}

	return &types.MsgUpdateKeysResponse{}, nil
}

func (k msgServer) updateUserKeys(ctx context.Context, msg *types.MsgUpdateKeys) error {
	creatorAddress, err := k.addressCodec.StringToBytes(msg.Creator)
	if err != nil {
		return errorsmod.Wrap(types.ErrInvalidCreatorAddres, "")
	}

	exists, err := k.UserMinaToCosmosHas(ctx, msg.PrevMinaPublicKey)
	if err != nil {
		return err
	}
	if !exists {
		return types.ErrUserNotRegistered
	}

	cosmosAddress, err := k.UserGetMinaToCosmos(ctx, msg.PrevMinaPublicKey)
	if err != nil {
		return err
	}
	if !bytes.Equal(creatorAddress, cosmosAddress) {
		return errorsmod.Wrap(types.ErrInvalidSigner, "")
	}

	exists, err = k.UserCosmosToMinaHas(ctx, cosmosAddress)
	if err != nil {
		return err
	}
	if !exists {
		return types.ErrUserNotRegistered
	}

	currentMinaAddress, err := k.UserGetCosmosToMina(ctx, cosmosAddress)
	if err != nil {
		return err
	}
	if !bytes.Equal(currentMinaAddress, msg.PrevMinaPublicKey) {
		return types.ErrUserNotRegistered
	}

	exists, err = k.UserMinaToCosmosHas(ctx, msg.NewMinaPublicKey)
	if err != nil {
		return err
	}
	if exists && !bytes.Equal(msg.NewMinaPublicKey, msg.PrevMinaPublicKey) {
		return types.ErrUserSecondaryKeyExists
	}

	if !VerifyUserCosmosSig(string(msg.CosmosSignature), msg.NewMinaPublicKey, cosmosAddress) {
		return types.ErrInvalidSignature
	}
	if !VerifyUserMinaSig(string(msg.NewMinaSignature), cosmosAddress, msg.NewMinaPublicKey) {
		return types.ErrInvalidSignature
	}

	err = k.userMinaToCosmos.Remove(ctx, msg.PrevMinaPublicKey)
	if err != nil {
		return err
	}
	err = k.userCosmosToMina.Set(ctx, cosmosAddress, msg.NewMinaPublicKey)
	if err != nil {
		return err
	}

	return k.userMinaToCosmos.Set(ctx, msg.NewMinaPublicKey, cosmosAddress)
}

func (k msgServer) updateValidatorKeys(ctx context.Context, msg *types.MsgUpdateKeys) error {
	if _, err := sdk.ConsAddressFromBech32(msg.Creator); err != nil {
		return errorsmod.Wrap(types.ErrInvalidCreatorAddres, "")
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
	if deriveAddressFromPubkey(cosmosPublicKey, false) != msg.Creator {
		return errorsmod.Wrap(types.ErrInvalidSigner, "")
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

	if !VerifyValidatorCosmosSig(string(msg.CosmosSignature), msg.NewMinaPublicKey, cosmosPublicKey) {
		return types.ErrInvalidSignature
	}
	if !VerifyValidatorMinaSig(string(msg.NewMinaSignature), cosmosPublicKey, msg.NewMinaPublicKey) {
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
