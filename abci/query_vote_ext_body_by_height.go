package abci

import (
	"context"
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ QueryServer = (*ABCIHandler)(nil)

func (h *ABCIHandler) VoteExtBodyByHeight(ctx context.Context, req *QueryVoteExtBodyByHeightRequest) (*QueryVoteExtBodyByHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	voteExtensionHeight := req.GetVoteExtensionHeight()
	if voteExtensionHeight < MinPulsarVoteExtensionHeight {
		return nil, status.Errorf(codes.InvalidArgument, "there is no vote extension body for heights smaller than %d", MinPulsarVoteExtensionHeight)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if voteExtensionHeight >= sdkCtx.BlockHeight() {
		return nil, status.Error(codes.InvalidArgument, "no vote extension body for the requested height yet")
	}

	voteExtBody, err := h.constructVoteExtBody(sdkCtx, voteExtensionHeight)
	if err != nil {
		return nil, voteExtBodyQueryError(err)
	}

	return &QueryVoteExtBodyByHeightResponse{
		VoteExtBody: &voteExtBody,
	}, nil
}

func voteExtBodyQueryError(err error) error {
	if errors.Is(err, ErrValidatorMinaKeyNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}

	return status.Errorf(codes.Internal, "failed to construct vote extension body: %v", err)
}
