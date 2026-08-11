package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	errorsmod "cosmossdk.io/errors"
	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func (k msgServer) PushNewProofHash(ctx context.Context, msg *types.MsgPushNewProofHash) (*types.MsgPushNewProofHashResponse, error) {
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(err, "invalid authority address")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	err := k.Keeper.AppendPendingProof(ctx, msg.ProofHash, sdkCtx.BlockHeight())
	if err != nil {
		return nil, err
	}

	return &types.MsgPushNewProofHashResponse{}, nil
}
