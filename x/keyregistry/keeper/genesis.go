package keeper

import (
	"context"

	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {

	keyPairs := genState.KeyPairs

	// Insert genesis key pairs.
	for _, keyPair := range keyPairs {

		err := k.cosmosToMina.Set(ctx, keyPair.CosmosKey, keyPair.MinaKey)
		if err != nil {
			return err
		}
		err = k.minaToCosmos.Set(ctx, keyPair.MinaKey, keyPair.CosmosKey)
		if err != nil {
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

	var keyPairs []*types.KeyPair

	var existenceMap = make(map[string]bool)

	// Iterate over CosmosToMina map first and collect all key pairs.
	cosmosIterator, err := k.cosmosToMina.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer cosmosIterator.Close()

	for cosmosIterator.Valid() {
		cosmosKey, err := cosmosIterator.Key()
		if err != nil {
			return genesis, err
		}
		minaKey, err := cosmosIterator.Value()
		if err != nil {
			return genesis, err
		}
		keyPair := &types.KeyPair{
			MinaKey:   minaKey,
			CosmosKey: cosmosKey,
		}
		keyPairs = append(keyPairs, keyPair)
		existenceMap[keyPair.String()] = true
		cosmosIterator.Next()
	}

	// Iterate over MinaToCosmos map and collect any key pairs that are not
	// already present in the CosmosToMina map. Although both maps are expected
	// to be in sync, this ensures no key pairs are lost in case of any inconsistency
	// between the two maps during export.

	// ExportGenesis intentionally does not enforce consistency between the two maps.
	// Returning an error here could prevent the state from being exported and lead
	// to potential state loss. Consistency checks should instead be handled at the
	// message implementation level where the mappings are created or updated.
	minaIterator, err := k.minaToCosmos.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer minaIterator.Close()

	for minaIterator.Valid() {
		minaKey, err := minaIterator.Key()
		if err != nil {
			return genesis, err
		}
		cosmosKey, err := minaIterator.Value()
		if err != nil {
			return genesis, err
		}
		keyPair := &types.KeyPair{
			MinaKey:   minaKey,
			CosmosKey: cosmosKey,
		}
		if existenceMap[keyPair.String()] {
			minaIterator.Next()
			continue
		}
		keyPairs = append(keyPairs, keyPair)
		minaIterator.Next()
	}

	genesis.KeyPairs = keyPairs

	return genesis, nil
}
