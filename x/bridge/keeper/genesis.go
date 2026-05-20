package keeper

import (
	"context"

	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {

	err := k.BridgeState.Set(ctx, genState.BridgeState)
	if err != nil {
		return err
	}
	return k.Params.Set(ctx, genState.Params)
}

// ExportGenesis returns the module's exported genesis.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	var err error

	genesis := types.DefaultGenesis()
	genesis.Params, err = k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	genesis.BridgeState, err = k.GetBridgeState(ctx)
	if err != nil {
		return nil, err
	}

	return genesis, nil
}
