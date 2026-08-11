package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func (k msgServer) PushNewProofHash(ctx context.Context, msg *types.MsgPushNewProofHash) (*types.MsgPushNewProofHashResponse, error) {
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(err, "invalid authority address")
	}

	// TODO: Handle the message

	return &types.MsgPushNewProofHashResponse{}, nil
}
