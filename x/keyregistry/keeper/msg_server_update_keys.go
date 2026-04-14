package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func (k msgServer) UpdateKeys(ctx context.Context, msg *types.MsgUpdateKeys) (*types.MsgUpdateKeysResponse, error) {
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(err, "invalid authority address")
	}

	if msg.UpdateType == types.KeyUpdateType_USER {

		exists, err := k.UserCosmosToMinaHas(ctx, []byte(msg.Creator))
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, types.ErrUserNotRegistered
		}
		exists, err = k.UserMinaToCosmosHas(ctx, msg.PrevMinaPublicKey)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, types.ErrUserNotRegistered
		}

		if !VerifyUserMinaSig(string(msg.NewMinaSignature), []byte(msg.Creator), msg.NewMinaPublicKey) {
			return nil, types.ErrInvalidSignature
		}

		err = k.userMinaToCosmos.Remove(ctx, msg.PrevMinaPublicKey)
		if err != nil {
			return nil, err
		}
		err = k.userCosmosToMina.Remove(ctx, []byte(msg.Creator))
		if err != nil {
			return nil, err
		}

		err = k.userCosmosToMina.Set(ctx, []byte(msg.Creator), msg.NewMinaPublicKey)
		if err != nil {
			return nil, err
		}
		err = k.userMinaToCosmos.Set(ctx, msg.NewMinaPublicKey, []byte(msg.Creator))
		if err != nil {
			return nil, err
		}
	}

	if msg.UpdateType == types.KeyUpdateType_VALIDATOR {

		exists, err := k.ValidatorCosmosToMinaHas(ctx, []byte(msg.Creator))
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, types.ErrValidatorNotRegistered
		}

		exists, err = k.ValidatorMinaToCosmosHas(ctx, msg.PrevMinaPublicKey)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, types.ErrValidatorNotRegistered
		}

		if !VerifyValidatorMinaSig(string(msg.NewMinaSignature), []byte(msg.Creator), msg.NewMinaPublicKey) {
			return nil, types.ErrInvalidSignature
		}

		err = k.validatorMinaToCosmos.Remove(ctx, msg.PrevMinaPublicKey)
		if err != nil {
			return nil, err
		}
		err = k.validatorCosmosToMina.Remove(ctx, []byte(msg.Creator))
		if err != nil {
			return nil, err
		}

		err = k.validatorCosmosToMina.Set(ctx, []byte(msg.Creator), msg.NewMinaPublicKey)
		if err != nil {
			return nil, err
		}
		err = k.validatorMinaToCosmos.Set(ctx, msg.NewMinaPublicKey, []byte(msg.Creator))
		if err != nil {
			return nil, err
		}
	}

	return &types.MsgUpdateKeysResponse{}, nil
}
