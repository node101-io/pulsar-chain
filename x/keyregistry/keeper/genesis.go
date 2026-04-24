package keeper

import (
	"context"

	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {

	// Insert genesis key pairs.
	for _, keyPair := range genState.UserCosmosToMina {
		err := k.userCosmosToMina.Set(ctx, keyPair.CosmosKey, keyPair.MinaKey)
		if err != nil {
			return err
		}
	}
	for _, keyPair := range genState.UserMinaToCosmos {
		err := k.userMinaToCosmos.Set(ctx, keyPair.MinaKey, keyPair.CosmosKey)
		if err != nil {
			return err
		}
	}
	for _, keyPair := range genState.ValidatorCosmosToMina {
		err := k.validatorCosmosToMina.Set(ctx, keyPair.CosmosKey, keyPair.MinaKey)
		if err != nil {
			return err
		}
	}
	for _, keyPair := range genState.ValidatorMinaToCosmos {
		err := k.validatorMinaToCosmos.Set(ctx, keyPair.MinaKey, keyPair.CosmosKey)
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

	userCosmosToMinaIterator, err := k.userCosmosToMina.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}

	defer userCosmosToMinaIterator.Close()

	var userCosmosToMinaSlice []*types.UserPublicKeyPair

	for userCosmosToMinaIterator.Valid() {
		cosmosKey, err := userCosmosToMinaIterator.Key()
		if err != nil {
			return nil, err
		}
		minaKey, err := userCosmosToMinaIterator.Value()
		if err != nil {
			return nil, err
		}
		keyPair := &types.UserPublicKeyPair{
			MinaKey:   minaKey,
			CosmosKey: cosmosKey,
		}

		userCosmosToMinaSlice = append(userCosmosToMinaSlice, keyPair)

		userCosmosToMinaIterator.Next()
	}

	genesis.UserCosmosToMina = userCosmosToMinaSlice

	userMinaToCosmosIterator, err := k.userMinaToCosmos.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}

	defer userMinaToCosmosIterator.Close()

	var userMinaToCosmosSlice []*types.UserPublicKeyPair

	for userMinaToCosmosIterator.Valid() {
		minaKey, err := userMinaToCosmosIterator.Key()
		if err != nil {
			return nil, err
		}
		cosmosKey, err := userMinaToCosmosIterator.Value()
		if err != nil {
			return nil, err
		}
		keyPair := &types.UserPublicKeyPair{
			MinaKey:   minaKey,
			CosmosKey: cosmosKey,
		}

		userMinaToCosmosSlice = append(userMinaToCosmosSlice, keyPair)

		userMinaToCosmosIterator.Next()
	}

	genesis.UserMinaToCosmos = userMinaToCosmosSlice

	validatorCosmosToMinaIterator, err := k.validatorCosmosToMina.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}

	defer validatorCosmosToMinaIterator.Close()

	var validatorCosmosToMinaSlice []*types.ValidatorPublicKeyPair

	for validatorCosmosToMinaIterator.Valid() {
		cosmosKey, err := validatorCosmosToMinaIterator.Key()
		if err != nil {
			return nil, err
		}
		minaKey, err := validatorCosmosToMinaIterator.Value()
		if err != nil {
			return nil, err
		}
		keyPair := &types.ValidatorPublicKeyPair{
			MinaKey:   minaKey,
			CosmosKey: cosmosKey,
		}

		validatorCosmosToMinaSlice = append(validatorCosmosToMinaSlice, keyPair)

		validatorCosmosToMinaIterator.Next()
	}

	genesis.ValidatorCosmosToMina = validatorCosmosToMinaSlice

	validatorMinaToCosmosIterator, err := k.validatorMinaToCosmos.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}

	defer validatorMinaToCosmosIterator.Close()

	var validatorMinaToCosmosSlice []*types.ValidatorPublicKeyPair

	for validatorMinaToCosmosIterator.Valid() {
		minaKey, err := validatorMinaToCosmosIterator.Key()
		if err != nil {
			return nil, err
		}
		cosmosKey, err := validatorMinaToCosmosIterator.Value()
		if err != nil {
			return nil, err
		}
		keyPair := &types.ValidatorPublicKeyPair{
			MinaKey:   minaKey,
			CosmosKey: cosmosKey,
		}

		validatorMinaToCosmosSlice = append(validatorMinaToCosmosSlice, keyPair)

		validatorMinaToCosmosIterator.Next()
	}

	genesis.ValidatorMinaToCosmos = validatorMinaToCosmosSlice

	return genesis, nil
}
