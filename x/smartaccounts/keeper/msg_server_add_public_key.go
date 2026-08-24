package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"
)

func (k msgServer) AddPublicKey(ctx context.Context, msg *types.MsgAddPublicKey) (*types.MsgAddPublicKeyResponse, error) {
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(err, "invalid authority address")
	}

	// TODO: Handle the message

	return &types.MsgAddPublicKeyResponse{}, nil
}
