package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func (k msgServer) PushNewProofHash(ctx context.Context, msg *types.MsgPushNewProofHash) (*types.MsgPushNewProofHashResponse, error) {
	if msg == nil {
		return nil, status.Error(codes.InvalidArgument, types.ErrInvalidPushNewProofHashRequest.Error())
	}

	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s: %v", types.ErrInvalidCreatorAddress, err)
	}

	if len(msg.ProofHash) != types.ProofHashLength {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"%s: got %d bytes",
			types.ErrInvalidProofHashLength,
			len(msg.ProofHash),
		)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	err := k.Keeper.AppendPendingProof(ctx, msg.ProofHash, sdkCtx.BlockHeight())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s: %v", types.ErrFailedToAppendPendingProof, err)
	}

	return &types.MsgPushNewProofHashResponse{}, nil
}
