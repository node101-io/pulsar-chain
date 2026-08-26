package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) GetSessionKeysByIdentity(ctx context.Context, req *types.QueryGetSessionKeysByIdentityRequest) (*types.QueryGetSessionKeysByIdentityResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if len(req.Identity) == 0 {
		return nil, status.Error(codes.InvalidArgument, "empty identity")
	}

	if len(req.Identity) != types.IdentitySize {
		return nil, status.Error(codes.InvalidArgument, "invalid identity size")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	exists, err := q.k.HasSmartAccount(sdkCtx, req.Identity)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	if !exists {
		return nil, status.Error(codes.NotFound, "identity not found")
	}

	acc, err := q.k.smartAccounts.Get(sdkCtx, req.Identity)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	var sessionKeys []*types.SessionKey
	for _, key := range acc.SessionKeys {
		sessionKeys = append(sessionKeys, &key)
	}

	return &types.QueryGetSessionKeysByIdentityResponse{
		AccountAddress: acc.AccountAddress,
		SessionKeys:    sessionKeys,
	}, nil
}
