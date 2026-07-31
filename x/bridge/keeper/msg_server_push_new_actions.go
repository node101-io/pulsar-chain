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

	span := msg.MinaBlockHeight - bridgeState.LatestFetchedMinaHeight
	if span > params.MaxBlockRange {
		return nil, types.ErrMinaBlockRangeTooLarge
	}

	if k.archiveWrapperClient == nil {
		return nil, types.ErrArchiveWrapperQueryClientNotConfigured
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

	target := msg.MinaBlockHeight
	actions, err := k.archiveWrapperClient.GetActionsInRange(ctx, bridgeState.LatestFetchedMinaHeight, target)
	if err != nil {
		return nil, err
	}

	// An out-of-range action makes the wrapper response untrustworthy.
	// Reject the batch before applying actions or advancing the cursor.
	for _, act := range actions {
		if act.BlockHeight <= bridgeState.LatestFetchedMinaHeight ||
			act.BlockHeight > target {
			return nil, types.ErrActionOutsideRequestedRange
		}
	}

	currentRoot, err := k.Keeper.GetLatestActionsReducedRoot(ctx)
	if err != nil {
		return nil, err
	}

	list, err := merkle.NewMerkleListFromRoot(types.ActionsReducedRootMerkleListPrefixV1, currentRoot)
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

		fieldElement, err := act.ToFieldElement()
		if err != nil {
			return nil, err
		}

		if err := list.Append(fieldElement.Bytes()); err != nil {
			return nil, err
		}
	}

	newRoot := list.Root()

	if err := k.Keeper.BridgeState.Set(ctx, types.BridgeState{
		LatestFetchedMinaHeight:   bridgeState.LatestFetchedMinaHeight,
		CurrentActionsReducedRoot: bridgeState.CurrentActionsReducedRoot,
	}); err != nil {
		return nil, err
	}

	if err := k.Keeper.setActionsReducedRootSnapshot(ctx, sdkCtx.BlockHeight(), newRoot); err != nil {
		return nil, err
	}

	return &types.MsgPushNewActionsResponse{}, nil
}
