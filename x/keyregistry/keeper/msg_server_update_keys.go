package keeper

import (
	"bytes"
	"context"

	"cosmossdk.io/errors"
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func (k msgServer) UpdateKeys(ctx context.Context, msg *types.MsgUpdateKeys) (*types.MsgUpdateKeysResponse, error) {
	var err error

	if len(msg.NewMinaPublicKey) != keys.PublicKeyTotalByteSize {
		return nil, errors.Wrap(types.ErrInvalidPublicKey, "")
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
		return errorsmod.Wrap(types.ErrInvalidCreatorAddres, "")
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
		return errorsmod.Wrap(types.ErrInvalidSigner, "")
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

	if !VerifyUserCosmosSig(string(msg.CosmosSignature), msg.NewMinaPublicKey, cosmosPublicKey) {
		return types.ErrInvalidSignature
	}
	if !VerifyUserMinaSig(string(msg.NewMinaSignature), cosmosPublicKey, msg.NewMinaPublicKey) {
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
