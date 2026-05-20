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

	currentMinaBlockHeight, err := GetMinaBlockHeight()
	if err != nil {
		return nil, err
	}

	bridgeState, err := k.Keeper.GetBridgeState(ctx)
	if err != nil {
		return nil, err
	}

	if currentMinaBlockHeight-msg.MinaBlockHeight < types.MINA_HARDFINALITY_BLOCK_DURATION {
		return nil, types.ErrMinaBlockNotFinalized
	}

	actions, err := fetchActions(types.ContractAddress, bridgeState.LatestFetchedMinaHeight, msg.MinaBlockHeight)
	if err != nil {
		return nil, err
	}
	for _, act := range actions {

		valid, err := k.isValid(ctx, &act)
		if err != nil {
			return nil, err
		}
		if !valid {
			continue
		}
		if err := k.apply(ctx, &act); err != nil {
			return nil, err
		}
	}

	if err := k.Keeper.setBridgeState(ctx, types.BridgeState{
		LatestFetchedMinaHeight: msg.MinaBlockHeight,
		ActionsReducedRoot:      []byte(""),
	}); err != nil {
		return nil, err
	}

	return &types.MsgPushNewActionsResponse{}, nil
}
