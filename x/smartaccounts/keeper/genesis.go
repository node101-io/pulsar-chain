package keeper

import (
	"context"

	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {
	if err := genState.Validate(); err != nil {
		return err
	}

	for _, entry := range genState.SmartAccounts {
		if err := k.smartAccounts.Set(ctx, entry.Identity, entry.Account); err != nil {
			return err
		}
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

	iter, err := k.smartAccounts.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {

		acc, err := iter.Value()
		if err != nil {
			iter.Next()
		}
		identity, err := iter.Key()
		if err != nil {
			iter.Next()
		}

		genesis.SmartAccounts = append(genesis.SmartAccounts, types.SmartAccountEntry{
			Identity: identity,
			Account:  acc,
		})

	}

	if err := genesis.Validate(); err != nil {
		return nil, err
	}

	return genesis, nil
}
