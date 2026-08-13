package keeper

import (
	"bytes"
	"context"
	"math"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func (k msgServer) UpdateUserKeys(ctx context.Context, msg *types.MsgUpdateUserKeys) (*types.MsgUpdateUserKeysResponse, error) {
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(types.ErrInvalidCreatorAddress, "creator address must be valid")
	}
	if err := types.ValidateUserCosmosPublicKey(msg.CosmosPublicKey); err != nil {
		return nil, err
	}
	if err := types.ValidateMinaPublicKey(msg.NewMinaPublicKey); err != nil {
		return nil, err
	}
	if msg.Creator != deriveUserAddress(msg.CosmosPublicKey) {
		return nil, errorsmod.Wrap(types.ErrInvalidCreatorAddress, "creator does not match cosmos public key")
	}

	currentMinaPublicKey, currentVersion, err := k.userCurrentKeyState(ctx, msg.CosmosPublicKey)
	if err != nil {
		return nil, err
	}
	if err := validateKeyUpdate(currentMinaPublicKey, msg.NewMinaPublicKey, currentVersion, msg.NewKeyVersion); err != nil {
		return nil, err
	}
	if exists, err := k.userMinaToCosmos.Has(ctx, msg.NewMinaPublicKey); err != nil {
		return nil, err
	} else if exists {
		return nil, types.ErrUserSecondaryKeyExists
	}

	challenge, err := types.BuildKeySigningChallenge(types.KeySigningChallengeInput{
		ChainID:              chainID(ctx),
		Operation:            types.KeySigningOperation_KEY_SIGNING_OPERATION_UPDATE,
		ActorType:            types.ActorType_USER,
		CosmosPublicKey:      msg.CosmosPublicKey,
		CurrentMinaPublicKey: currentMinaPublicKey,
		NewMinaPublicKey:     msg.NewMinaPublicKey,
		NewKeyVersion:        msg.NewKeyVersion,
	})
	if err != nil {
		return nil, err
	}
	valid, err := verifyMinaFieldSignature(msg.NewMinaSignature, challenge, msg.NewMinaPublicKey)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, types.ErrInvalidSignature
	}

	if err := k.userMinaToCosmos.Remove(ctx, currentMinaPublicKey); err != nil {
		return nil, err
	}
	if err := k.userCosmosToMina.Set(ctx, msg.CosmosPublicKey, msg.NewMinaPublicKey); err != nil {
		return nil, err
	}
	if err := k.userMinaToCosmos.Set(ctx, msg.NewMinaPublicKey, msg.CosmosPublicKey); err != nil {
		return nil, err
	}
	if err := k.userKeyVersion.Set(ctx, msg.CosmosPublicKey, msg.NewKeyVersion); err != nil {
		return nil, err
	}

	return &types.MsgUpdateUserKeysResponse{}, nil
}

func (k msgServer) UpdateValidatorKeys(ctx context.Context, msg *types.MsgUpdateValidatorKeys) (*types.MsgUpdateValidatorKeysResponse, error) {
	if _, err := sdk.AccAddressFromBech32(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(types.ErrInvalidCreatorAddress, "creator address must be valid")
	}
	if err := types.ValidateValidatorCosmosPublicKey(msg.ValidatorConsensusPublicKey); err != nil {
		return nil, err
	}
	if err := types.ValidateMinaPublicKey(msg.NewMinaPublicKey); err != nil {
		return nil, err
	}

	currentMinaPublicKey, currentVersion, err := k.validatorCurrentKeyState(ctx, msg.ValidatorConsensusPublicKey)
	if err != nil {
		return nil, err
	}
	if err := validateKeyUpdate(currentMinaPublicKey, msg.NewMinaPublicKey, currentVersion, msg.NewKeyVersion); err != nil {
		return nil, err
	}
	if exists, err := k.validatorMinaToCosmos.Has(ctx, msg.NewMinaPublicKey); err != nil {
		return nil, err
	} else if exists {
		return nil, types.ErrValidatorSecondaryKeyExists
	}

	challenge, err := types.BuildKeySigningChallenge(types.KeySigningChallengeInput{
		ChainID:              chainID(ctx),
		Operation:            types.KeySigningOperation_KEY_SIGNING_OPERATION_UPDATE,
		ActorType:            types.ActorType_VALIDATOR,
		CosmosPublicKey:      msg.ValidatorConsensusPublicKey,
		CurrentMinaPublicKey: currentMinaPublicKey,
		NewMinaPublicKey:     msg.NewMinaPublicKey,
		NewKeyVersion:        msg.NewKeyVersion,
	})
	if err != nil {
		return nil, err
	}
	valid, err := verifyMinaFieldSignature(msg.NewMinaSignature, challenge, msg.NewMinaPublicKey)
	if err != nil {
		return nil, err
	}
	if !valid || !verifyValidatorConsensusSignature(msg.ValidatorConsensusSignature, challenge.Bytes(), msg.ValidatorConsensusPublicKey) {
		return nil, types.ErrInvalidSignature
	}

	if err := k.validatorMinaToCosmos.Remove(ctx, currentMinaPublicKey); err != nil {
		return nil, err
	}
	if err := k.validatorCosmosToMina.Set(ctx, msg.ValidatorConsensusPublicKey, msg.NewMinaPublicKey); err != nil {
		return nil, err
	}
	if err := k.validatorMinaToCosmos.Set(ctx, msg.NewMinaPublicKey, msg.ValidatorConsensusPublicKey); err != nil {
		return nil, err
	}
	if err := k.validatorKeyVersion.Set(ctx, msg.ValidatorConsensusPublicKey, msg.NewKeyVersion); err != nil {
		return nil, err
	}

	return &types.MsgUpdateValidatorKeysResponse{}, nil
}

func (k msgServer) userCurrentKeyState(ctx context.Context, cosmosPublicKey []byte) ([]byte, uint64, error) {
	exists, err := k.userCosmosToMina.Has(ctx, cosmosPublicKey)
	if err != nil {
		return nil, 0, err
	}
	if !exists {
		return nil, 0, types.ErrUserNotRegistered
	}
	current, err := k.userCosmosToMina.Get(ctx, cosmosPublicKey)
	if err != nil {
		return nil, 0, err
	}
	version, err := k.userKeyVersion.Get(ctx, cosmosPublicKey)
	return current, version, err
}

func (k msgServer) validatorCurrentKeyState(ctx context.Context, consensusPublicKey []byte) ([]byte, uint64, error) {
	exists, err := k.validatorCosmosToMina.Has(ctx, consensusPublicKey)
	if err != nil {
		return nil, 0, err
	}
	if !exists {
		return nil, 0, types.ErrValidatorNotRegistered
	}
	current, err := k.validatorCosmosToMina.Get(ctx, consensusPublicKey)
	if err != nil {
		return nil, 0, err
	}
	version, err := k.validatorKeyVersion.Get(ctx, consensusPublicKey)
	return current, version, err
}

func validateKeyUpdate(current, next []byte, currentVersion, requestedVersion uint64) error {
	if bytes.Equal(current, next) {
		return types.ErrUnchangedMinaPublicKey
	}
	if currentVersion == math.MaxUint64 {
		return errorsmod.Wrap(types.ErrInvalidKeyVersion, "key version exhausted")
	}
	if requestedVersion != currentVersion+1 {
		return errorsmod.Wrapf(types.ErrStaleKeyVersion, "got %d, expected %d", requestedVersion, currentVersion+1)
	}
	return nil
}
