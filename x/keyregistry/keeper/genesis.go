package keeper

import (
	"context"

	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {

	userKeyPairs := genState.UserKeyPairs

	// Insert genesis key pairs.
	for _, keyPair := range userKeyPairs {

		err := k.userCosmosToMina.Set(ctx, keyPair.CosmosKey, keyPair.MinaKey)
		if err != nil {
			return err
		}
		err = k.userMinaToCosmos.Set(ctx, keyPair.MinaKey, keyPair.CosmosKey)
		if err != nil {
			return err
		}
	}

	validatorKeyPairs := genState.ValidatorKeyPairs

	// Insert genesis key pairs.
	for _, keyPair := range validatorKeyPairs {

		err := k.validatorCosmosToMina.Set(ctx, keyPair.CosmosKey, keyPair.MinaKey)
		if err != nil {
			return err
		}
		err = k.validatorMinaToCosmos.Set(ctx, keyPair.MinaKey, keyPair.CosmosKey)
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

	var userKeyPairs []*types.KeyPair

	var userKeypairExistenceMap = make(map[string]bool)

	// Iterate over CosmosToMina map first and collect all key pairs.
	userCosmosIterator, err := k.userCosmosToMina.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer userCosmosIterator.Close()

	for userCosmosIterator.Valid() {
		cosmosKey, err := userCosmosIterator.Key()
		if err != nil {
			return genesis, err
		}
		minaKey, err := userCosmosIterator.Value()
		if err != nil {
			return genesis, err
		}
		keyPair := &types.KeyPair{
			MinaKey:   minaKey,
			CosmosKey: cosmosKey,
		}
		userKeyPairs = append(userKeyPairs, keyPair)
		userKeypairExistenceMap[keyPair.String()] = true
		userCosmosIterator.Next()
	}

	// Iterate over inaToCosmos map and collect any key pairs that are not
	// already present in the CosmosToMina map. Although both maps are expected
	// to be in sync, this ensures no key pairs are lost in case of any inconsistency
	// between the two maps during export.

	// ExportGenesis intentionally does not enforce consistency between the two maps.
	// Returning an error here could prevent the state from being exported and lead
	// to potential state loss. Consistency checks should instead be handled at the
	// message implementation level where the mappings are created or updated.
	userMinaIterator, err := k.userMinaToCosmos.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer userMinaIterator.Close()

	for userMinaIterator.Valid() {
		minaKey, err := userMinaIterator.Key()
		if err != nil {
			return genesis, err
		}
		cosmosKey, err := userMinaIterator.Value()
		if err != nil {
			return genesis, err
		}
		keyPair := &types.KeyPair{
			MinaKey:   minaKey,
			CosmosKey: cosmosKey,
		}
		if userKeypairExistenceMap[keyPair.String()] {
			userMinaIterator.Next()
			continue
		}
		userKeyPairs = append(userKeyPairs, keyPair)
		userMinaIterator.Next()
	}

	genesis.UserKeyPairs = userKeyPairs

	var validatorKeyPairs []*types.KeyPair

	var validatorKeypairExistenceMap = make(map[string]bool)

	// Iterate over CosmosToMina map first and collect all key pairs.
	validatorCosmosIterator, err := k.validatorCosmosToMina.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer validatorCosmosIterator.Close()

	for validatorCosmosIterator.Valid() {
		cosmosKey, err := validatorCosmosIterator.Key()
		if err != nil {
			return genesis, err
		}
		minaKey, err := validatorCosmosIterator.Value()
		if err != nil {
			return genesis, err
		}
		keyPair := &types.KeyPair{
			MinaKey:   minaKey,
			CosmosKey: cosmosKey,
		}
		validatorKeyPairs = append(validatorKeyPairs, keyPair)
		validatorKeypairExistenceMap[keyPair.String()] = true
		validatorCosmosIterator.Next()
	}

	validatorMinaIterator, err := k.validatorMinaToCosmos.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer validatorMinaIterator.Close()

	for validatorMinaIterator.Valid() {
		minaKey, err := validatorMinaIterator.Key()
		if err != nil {
			return genesis, err
		}
		cosmosKey, err := validatorMinaIterator.Value()
		if err != nil {
			return genesis, err
		}
		keyPair := &types.KeyPair{
			MinaKey:   minaKey,
			CosmosKey: cosmosKey,
		}
		if validatorKeypairExistenceMap[keyPair.String()] {
			validatorMinaIterator.Next()
			continue
		}
		validatorKeyPairs = append(validatorKeyPairs, keyPair)
		validatorMinaIterator.Next()
	}

	genesis.ValidatorKeyPairs = validatorKeyPairs

	return genesis, nil
}
