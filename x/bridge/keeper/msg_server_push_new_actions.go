package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

func (k msgServer) PushNewActions(ctx context.Context, msg *types.MsgPushNewActions) (*types.MsgPushNewActionsResponse, error) {
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(err, "invalid authority address")
	}

	// TODO: Handle the message

	return &types.MsgPushNewActionsResponse{}, nil
}
