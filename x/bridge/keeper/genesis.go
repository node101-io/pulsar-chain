package keeper

import (
	"bytes"
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {
	if err := genState.Validate(); err != nil {
		return err
	}

	if err := k.BridgeState.Set(ctx, genState.BridgeState); err != nil {
		return err
	}

	for _, snapshot := range genState.ActionsReducedRootSnapshots {
		if err := k.ActionsReducedRootSnapshots.Set(ctx, snapshot.CosmosBlockHeight, snapshot.ActionsReducedRoot); err != nil {
			return err
		}
	}

	return k.Params.Set(ctx, genState.Params)
}

func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	genesis := types.DefaultGenesis()

	params, err := k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	genesis.Params = params

	bridgeState, err := k.GetBridgeState(ctx)
	if err != nil {
		return nil, err
	}
	genesis.BridgeState = bridgeState
	genesis.ActionsReducedRootSnapshots = nil

	iter, err := k.ActionsReducedRootSnapshots.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		height, err := iter.Key()
		if err != nil {
			return nil, err
		}

		root, err := iter.Value()
		if err != nil {
			return nil, err
		}

		genesis.ActionsReducedRootSnapshots = append(genesis.ActionsReducedRootSnapshots, types.ActionsReducedRootSnapshot{
			CosmosBlockHeight:  height,
			ActionsReducedRoot: root,
		})
	}

	return genesis, nil
}

// PrepareForZeroHeightGenesis resets the Cosmos-height-indexed snapshot window
// while preserving the cumulative actions root and Mina cursor.
func (k Keeper) PrepareForZeroHeightGenesis(ctx context.Context) error {
	currentRoot, err := k.GetLatestActionsReducedRoot(ctx)
	if err != nil {
		return err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	cacheCtx, write := sdkCtx.CacheContext()

	bridgeState, err := k.GetBridgeState(cacheCtx)
	if err != nil {
		return err
	}

	bridgeState.ValidActionHashes = nil
	bridgeState.ValidActionHashesCosmosBlockHeight = 0
	bridgeState.StartMinaHeight = bridgeState.LatestFetchedMinaHeight

	if err := k.BridgeState.Set(cacheCtx, bridgeState); err != nil {
		return err
	}

	if err := k.ActionsReducedRootSnapshots.Clear(cacheCtx, nil); err != nil {
		return err
	}
	if err := k.ActionsReducedRootSnapshots.Set(cacheCtx, 0, bytes.Clone(currentRoot)); err != nil {
		return err
	}

	write()
	return nil
}
