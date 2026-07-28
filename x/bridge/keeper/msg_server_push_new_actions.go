package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	merkle "github.com/node101-io/mina-signer-go/merklelist"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

func (k msgServer) PushNewActions(ctx context.Context, msg *types.MsgPushNewActions) (*types.MsgPushNewActionsResponse, error) {
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(err, "invalid authority address")
	}

	currentMinaBlockHeight, err := k.getWrapperMinaBlockHeight(ctx)
	if err != nil {
		return nil, err
	}

	bridgeState, err := k.Keeper.GetBridgeState(ctx)
	if err != nil {
		return nil, err
	}

	params, err := k.Keeper.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	if msg.MinaBlockHeight <= 0 {
		return nil, types.ErrInvalidMinaBlockHeight
	}

	if msg.MinaBlockHeight <= bridgeState.LatestFetchedMinaHeight {
		return nil, types.ErrMinaBlockHeightMustAdvance
	}

	finalityDepth := params.ConfirmationDepth
	if currentMinaBlockHeight < finalityDepth {
		return nil, types.ErrMinaBlockNotFinalized
	}

	maxFinalizedHeight := currentMinaBlockHeight - finalityDepth
	if msg.MinaBlockHeight > maxFinalizedHeight {
		return nil, types.ErrMinaBlockNotFinalized
	}

	actions, err := k.getWrapperActionsInRange(ctx, bridgeState.LatestFetchedMinaHeight, msg.MinaBlockHeight)
	if err != nil {
		return nil, err
	}

	list, err := merkle.NewMerkleListFromRoot(types.MerkleListPrefix, bridgeState.ActionsReducedRoot)
	if err != nil {
		return nil, err
	}

	for _, act := range actions {

		valid, err := k.isValidAction(ctx, act)
		if err != nil {
			return nil, err
		}
		if !valid {
			continue
		}
		if err := k.apply(ctx, act); err != nil {
			return nil, err
		}

		bz, err := act.Marshal()
		if err != nil {
			return nil, err
		}

		if err := list.Append(bz); err != nil {
			return nil, err
		}

	}

	if err := k.Keeper.BridgeState.Set(ctx, types.BridgeState{
		LatestFetchedMinaHeight: msg.MinaBlockHeight,
		ActionsReducedRoot:      list.Root(),
	}); err != nil {
		return nil, err
	}

	return &types.MsgPushNewActionsResponse{}, nil
}
