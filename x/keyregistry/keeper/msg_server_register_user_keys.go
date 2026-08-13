package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func (k msgServer) RegisterUserKeys(ctx context.Context, msg *types.MsgRegisterUserKeys) (*types.MsgRegisterUserKeysResponse, error) {
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(types.ErrInvalidCreatorAddress, "creator address must be valid")
	}
	if err := (types.UserPublicKeyPair{CosmosKey: msg.CosmosPublicKey, MinaKey: msg.MinaPublicKey}).Validate(); err != nil {
		return nil, err
	}
	if msg.Creator != deriveUserAddress(msg.CosmosPublicKey) {
		return nil, errorsmod.Wrap(types.ErrInvalidCreatorAddress, "creator does not match cosmos public key")
	}

	cosmosKeyExists, err := k.userCosmosToMina.Has(ctx, msg.CosmosPublicKey)
	if err != nil {
		return nil, err
	}
	minaKeyExists, err := k.userMinaToCosmos.Has(ctx, msg.MinaPublicKey)
	if err != nil {
		return nil, err
	}
	if cosmosKeyExists || minaKeyExists {
		return nil, types.ErrUserSecondaryKeyExists
	}

	challenge, err := types.BuildKeySigningChallenge(types.KeySigningChallengeInput{
		ChainID:          sdk.UnwrapSDKContext(ctx).ChainID(),
		Operation:        types.KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER,
		ActorType:        types.ActorType_USER,
		CosmosPublicKey:  msg.CosmosPublicKey,
		NewMinaPublicKey: msg.MinaPublicKey,
	})
	if err != nil {
		return nil, err
	}
	valid, err := verifyMinaFieldSignature(msg.MinaSignature, challenge, msg.MinaPublicKey)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, types.ErrInvalidSignature
	}

	if err := k.userCosmosToMina.Set(ctx, msg.CosmosPublicKey, msg.MinaPublicKey); err != nil {
		return nil, err
	}
	if err := k.userMinaToCosmos.Set(ctx, msg.MinaPublicKey, msg.CosmosPublicKey); err != nil {
		return nil, err
	}
	if err := k.userKeyVersion.Set(ctx, msg.CosmosPublicKey, 0); err != nil {
		return nil, err
	}

	return &types.MsgRegisterUserKeysResponse{}, nil
}
