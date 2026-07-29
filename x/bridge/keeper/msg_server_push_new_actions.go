package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	merkle "github.com/node101-io/mina-signer-go/merklelist"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

func (k msgServer) PushNewActions(ctx context.Context, msg *types.MsgPushNewActions) (*types.MsgPushNewActionsResponse, error) {
	if _, err := k.addressCodec.StringToBytes(msg.Creator); err != nil {
		return nil, errorsmod.Wrap(err, "invalid authority address")
	}

	bridgeState, err := k.Keeper.GetBridgeState(ctx)
	if err != nil {
		return nil, err
	}

	if msg.MinaBlockHeight <= 0 {
		return nil, types.ErrInvalidMinaBlockHeight
	}

	params, err := k.Keeper.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	if msg.MinaBlockHeight < params.StartBlockHeight {
		return nil, types.ErrInvalidMinaBlockRange
	}

	if msg.MinaBlockHeight <= bridgeState.LatestFetchedMinaHeight {
		return nil, types.ErrMinaBlockHeightMustAdvance
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	currentMinaBlockHeight, err := k.archiveWrapperClient.GetMinaBlockHeight(ctx)
	if err != nil {
		return nil, err
	}

	// Wrapper height is already the latest confirmed/indexed cursor.
	// Do not apply confirmation depth a second time in the chain layer.
	if msg.MinaBlockHeight > currentMinaBlockHeight {
		return nil, types.ErrMinaBlockNotFinalized
	}

	actions, err := k.archiveWrapperClient.GetActionsInRange(ctx, bridgeState.LatestFetchedMinaHeight, msg.MinaBlockHeight)
	if err != nil {
		return nil, err
	}

	currentRoot, err := k.Keeper.GetLatestActionsReducedRoot(ctx)
	if err != nil {
		return nil, err
	}

	list, err := merkle.NewMerkleListFromRoot(types.MerkleListPrefix, currentRoot)
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

	newRoot := list.Root()

	if err := k.Keeper.BridgeState.Set(ctx, types.BridgeState{
		LatestFetchedMinaHeight: msg.MinaBlockHeight,
	}); err != nil {
		return nil, err
	}

	if err := k.Keeper.ActionsReducedRootSnapshots.Set(ctx, sdkCtx.BlockHeight(), newRoot); err != nil {
		return nil, err
	}

	return &types.MsgPushNewActionsResponse{}, nil
}
