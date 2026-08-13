package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func (k msgServer) RegisterValidatorKeys(ctx context.Context, msg *types.MsgRegisterValidatorKeys) (*types.MsgRegisterValidatorKeysResponse, error) {
	if _, err := sdk.AccAddressFromBech32(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(types.ErrInvalidCreatorAddress, "creator address must be valid")
	}
	if err := (types.ValidatorPublicKeyPair{CosmosKey: msg.ValidatorConsensusPublicKey, MinaKey: msg.MinaPublicKey}).Validate(); err != nil {
		return nil, err
	}

	consensusKeyExists, err := k.validatorCosmosToMina.Has(ctx, msg.ValidatorConsensusPublicKey)
	if err != nil {
		return nil, err
	}
	minaKeyExists, err := k.validatorMinaToCosmos.Has(ctx, msg.MinaPublicKey)
	if err != nil {
		return nil, err
	}
	if consensusKeyExists || minaKeyExists {
		return nil, types.ErrValidatorSecondaryKeyExists
	}

	challenge, err := types.BuildKeySigningChallenge(types.KeySigningChallengeInput{
		ChainID:          sdk.UnwrapSDKContext(ctx).ChainID(),
		Operation:        types.KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER,
		ActorType:        types.ActorType_ACTOR_TYPE_VALIDATOR,
		CosmosPublicKey:  msg.ValidatorConsensusPublicKey,
		NewMinaPublicKey: msg.MinaPublicKey,
	})
	if err != nil {
		return nil, err
	}
	valid, err := verifyMinaFieldSignature(msg.MinaSignature, challenge, msg.MinaPublicKey)
	if err != nil {
		return nil, err
	}
	if !valid || !verifyValidatorConsensusSignature(msg.ValidatorConsensusSignature, challenge.Bytes(), msg.ValidatorConsensusPublicKey) {
		return nil, types.ErrInvalidSignature
	}

	if err := k.validatorCosmosToMina.Set(ctx, msg.ValidatorConsensusPublicKey, msg.MinaPublicKey); err != nil {
		return nil, err
	}
	if err := k.validatorMinaToCosmos.Set(ctx, msg.MinaPublicKey, msg.ValidatorConsensusPublicKey); err != nil {
		return nil, err
	}
	if err := k.validatorKeyVersion.Set(ctx, msg.ValidatorConsensusPublicKey, 0); err != nil {
		return nil, err
	}

	return &types.MsgRegisterValidatorKeysResponse{}, nil
}
