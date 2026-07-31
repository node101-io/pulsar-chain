package keeper

import (
	"context"

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
