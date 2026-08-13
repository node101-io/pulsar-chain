package keeper

import (
	"bytes"
	"context"
	"math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) GetKeySigningChallenge(ctx context.Context, req *types.QueryGetKeySigningChallengeRequest) (*types.QueryGetKeySigningChallengeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	if err := validateChallengeQueryKeys(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentMinaPublicKey, newKeyVersion, err := q.resolveChallengeState(sdkCtx, req)
	if err != nil {
		return nil, err
	}

	challenge, err := types.BuildKeySigningChallenge(types.KeySigningChallengeInput{
		ChainID:              sdkCtx.ChainID(),
		Operation:            req.Operation,
		ActorType:            req.ActorType,
		CosmosPublicKey:      req.CosmosPublicKey,
		CurrentMinaPublicKey: currentMinaPublicKey,
		NewMinaPublicKey:     req.NewMinaPublicKey,
		NewKeyVersion:        newKeyVersion,
	})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &types.QueryGetKeySigningChallengeResponse{
		Challenge:             challenge.String(),
		ChallengeBytes:        challenge.Bytes(),
		PreviousMinaPublicKey: currentMinaPublicKey,
		NewKeyVersion:         newKeyVersion,
	}, nil
}

func validateChallengeQueryKeys(req *types.QueryGetKeySigningChallengeRequest) error {
	switch req.ActorType {
	case types.ActorType_USER:
		if err := types.ValidateUserCosmosPublicKey(req.CosmosPublicKey); err != nil {
			return err
		}
	case types.ActorType_VALIDATOR:
		if err := types.ValidateValidatorCosmosPublicKey(req.CosmosPublicKey); err != nil {
			return err
		}
	default:
		return types.ErrInvalidActorType
	}
	return types.ValidateMinaPublicKey(req.NewMinaPublicKey)
}

func (q queryServer) resolveChallengeState(ctx context.Context, req *types.QueryGetKeySigningChallengeRequest) ([]byte, uint64, error) {
	switch req.Operation {
	case types.KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER:
		exists, err := q.actorStableKeyExists(ctx, req.ActorType, req.CosmosPublicKey)
		if err != nil {
			return nil, 0, status.Error(codes.Internal, "failed to read key registry state")
		}
		minaExists, err := q.actorMinaKeyExists(ctx, req.ActorType, req.NewMinaPublicKey)
		if err != nil {
			return nil, 0, status.Error(codes.Internal, "failed to read key registry state")
		}
		if exists || minaExists {
			return nil, 0, status.Error(codes.AlreadyExists, "key is already registered")
		}
		return nil, 0, nil

	case types.KeySigningOperation_KEY_SIGNING_OPERATION_UPDATE:
		current, version, err := q.actorCurrentKeyState(ctx, req.ActorType, req.CosmosPublicKey)
		if err != nil {
			return nil, 0, err
		}
		if bytes.Equal(current, req.NewMinaPublicKey) {
			return nil, 0, status.Error(codes.InvalidArgument, types.ErrUnchangedMinaPublicKey.Error())
		}
		minaExists, err := q.actorMinaKeyExists(ctx, req.ActorType, req.NewMinaPublicKey)
		if err != nil {
			return nil, 0, status.Error(codes.Internal, "failed to read key registry state")
		}
		if minaExists {
			return nil, 0, status.Error(codes.AlreadyExists, "new mina public key is already registered")
		}
		if version == math.MaxUint64 {
			return nil, 0, status.Error(codes.FailedPrecondition, "key version exhausted")
		}
		return current, version + 1, nil

	default:
		return nil, 0, status.Error(codes.InvalidArgument, types.ErrInvalidSigningOperation.Error())
	}
}

func (q queryServer) actorStableKeyExists(ctx context.Context, actor types.ActorType, key []byte) (bool, error) {
	if actor == types.ActorType_USER {
		return q.k.userCosmosToMina.Has(ctx, key)
	}
	return q.k.validatorCosmosToMina.Has(ctx, key)
}

func (q queryServer) actorMinaKeyExists(ctx context.Context, actor types.ActorType, key []byte) (bool, error) {
	if actor == types.ActorType_USER {
		return q.k.userMinaToCosmos.Has(ctx, key)
	}
	return q.k.validatorMinaToCosmos.Has(ctx, key)
}

func (q queryServer) actorCurrentKeyState(ctx context.Context, actor types.ActorType, stableKey []byte) ([]byte, uint64, error) {
	if actor == types.ActorType_USER {
		exists, err := q.k.userCosmosToMina.Has(ctx, stableKey)
		if err != nil {
			return nil, 0, status.Error(codes.Internal, "failed to read key registry state")
		}
		if !exists {
			return nil, 0, status.Error(codes.NotFound, types.ErrUserNotRegistered.Error())
		}
		current, err := q.k.userCosmosToMina.Get(ctx, stableKey)
		if err != nil {
			return nil, 0, status.Error(codes.Internal, "failed to read key registry state")
		}
		version, err := q.k.userKeyVersion.Get(ctx, stableKey)
		if err != nil {
			return nil, 0, status.Error(codes.Internal, "failed to read key version")
		}
		return current, version, nil
	}

	exists, err := q.k.validatorCosmosToMina.Has(ctx, stableKey)
	if err != nil {
		return nil, 0, status.Error(codes.Internal, "failed to read key registry state")
	}
	if !exists {
		return nil, 0, status.Error(codes.NotFound, types.ErrValidatorNotRegistered.Error())
	}
	current, err := q.k.validatorCosmosToMina.Get(ctx, stableKey)
	if err != nil {
		return nil, 0, status.Error(codes.Internal, "failed to read key registry state")
	}
	version, err := q.k.validatorKeyVersion.Get(ctx, stableKey)
	if err != nil {
		return nil, 0, status.Error(codes.Internal, "failed to read key version")
	}
	return current, version, nil
}
