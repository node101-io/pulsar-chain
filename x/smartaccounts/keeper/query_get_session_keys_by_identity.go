package keeper

import (
	"context"
	"encoding/binary"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxSessionKeysPageSize uint64 = 100

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

	pageRequest := req.Pagination

	var offset uint64
	limit := maxSessionKeysPageSize

	if pageRequest != nil {
		if pageRequest.Reverse {
			return nil, status.Error(
				codes.InvalidArgument,
				"reverse pagination is not supported",
			)
		}

		offset = pageRequest.Offset
		if len(pageRequest.Key) != 0 {
			if pageRequest.Offset != 0 {
				return nil, status.Error(
					codes.InvalidArgument,
					"pagination key and offset cannot both be set",
				)
			}
			if len(pageRequest.Key) != 8 {
				return nil, status.Error(codes.InvalidArgument, "invalid pagination key")
			}
			offset = binary.BigEndian.Uint64(pageRequest.Key)
		}

		if pageRequest.Limit > 0 && pageRequest.Limit < limit {
			limit = pageRequest.Limit
		}
	}

	total := uint64(len(acc.SessionKeys))

	if offset > total {
		offset = total
	}

	end := total
	if limit < total-offset {
		end = offset + limit
	}

	sessionKeys := make([]*types.SessionKey, 0, int(end-offset))
	for _, key := range acc.SessionKeys[offset:end] {
		sessionKeys = append(sessionKeys, &key)
	}

	pageResponse := &query.PageResponse{Total: total}
	if end < total {
		pageResponse.NextKey = make([]byte, 8)
		binary.BigEndian.PutUint64(pageResponse.NextKey, end)
	}

	return &types.QueryGetSessionKeysByIdentityResponse{
		AccountAddress: acc.AccountAddress,
		SessionKeys:    sessionKeys,
		Pagination:     pageResponse,
	}, nil
}
