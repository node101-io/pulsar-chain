package keeper

import (
	"bytes"
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func (k Keeper) InitGenesis(ctx context.Context, genState types.GenesisState) error {
	if err := genState.Validate(); err != nil {
		return err
	}

	for _, keyPair := range genState.UserKeyPairs {
		err := k.userCosmosToMina.Set(ctx, keyPair.CosmosKey, keyPair.MinaKey)
		if err != nil {
			return err
		}
		err = k.userMinaToCosmos.Set(ctx, keyPair.MinaKey, keyPair.CosmosKey)
		if err != nil {
			return err
		}
	}

	for _, keyPair := range genState.ValidatorKeyPairs {
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

	genesis.UserKeyPairs, err = k.exportUserGenesisKeyPairs(ctx)
	if err != nil {
		return nil, err
	}

	genesis.ValidatorKeyPairs, err = k.exportValidatorGenesisKeyPairs(ctx)
	if err != nil {
		return nil, err
	}

	if err := genesis.Validate(); err != nil {
		return nil, errorsmod.Wrap(types.ErrInvalidGenesisState, err.Error())
	}

	return genesis, nil
}

func (k Keeper) exportUserGenesisKeyPairs(ctx context.Context) ([]*types.UserPublicKeyPair, error) {
	userCosmosToMinaIterator, err := k.userCosmosToMina.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer userCosmosToMinaIterator.Close()

	var userKeyPairs []*types.UserPublicKeyPair

	for userCosmosToMinaIterator.Valid() {
		cosmosKey, err := userCosmosToMinaIterator.Key()
		if err != nil {
			return nil, err
		}
		minaKey, err := userCosmosToMinaIterator.Value()
		if err != nil {
			return nil, err
		}
		reverseCosmosKey, err := k.userMinaToCosmos.Get(ctx, minaKey)
		if err != nil {
			return nil, errorsmod.Wrap(types.ErrInvalidGenesisState, "user key pair missing reverse index")
		}
		if !bytes.Equal(reverseCosmosKey, cosmosKey) {
			return nil, errorsmod.Wrap(types.ErrInvalidGenesisState, "user key pair reverse index mismatch")
		}

		userKeyPairs = append(userKeyPairs, &types.UserPublicKeyPair{
			MinaKey:   minaKey,
			CosmosKey: cosmosKey,
		})

		userCosmosToMinaIterator.Next()
	}

	userMinaToCosmosIterator, err := k.userMinaToCosmos.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer userMinaToCosmosIterator.Close()

	for userMinaToCosmosIterator.Valid() {
		minaKey, err := userMinaToCosmosIterator.Key()
		if err != nil {
			return nil, err
		}
		cosmosKey, err := userMinaToCosmosIterator.Value()
		if err != nil {
			return nil, err
		}
		forwardMinaKey, err := k.userCosmosToMina.Get(ctx, cosmosKey)
		if err != nil {
			return nil, errorsmod.Wrap(types.ErrInvalidGenesisState, "user reverse key pair missing forward index")
		}
		if !bytes.Equal(forwardMinaKey, minaKey) {
			return nil, errorsmod.Wrap(types.ErrInvalidGenesisState, "user reverse key pair forward index mismatch")
		}

		userMinaToCosmosIterator.Next()
	}

	return userKeyPairs, nil
}

func (k Keeper) exportValidatorGenesisKeyPairs(ctx context.Context) ([]*types.ValidatorPublicKeyPair, error) {
	validatorCosmosToMinaIterator, err := k.validatorCosmosToMina.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer validatorCosmosToMinaIterator.Close()

	var validatorKeyPairs []*types.ValidatorPublicKeyPair

	for validatorCosmosToMinaIterator.Valid() {
		cosmosKey, err := validatorCosmosToMinaIterator.Key()
		if err != nil {
			return nil, err
		}
		minaKey, err := validatorCosmosToMinaIterator.Value()
		if err != nil {
			return nil, err
		}
		reverseCosmosKey, err := k.validatorMinaToCosmos.Get(ctx, minaKey)
		if err != nil {
			return nil, errorsmod.Wrap(types.ErrInvalidGenesisState, "validator key pair missing reverse index")
		}
		if !bytes.Equal(reverseCosmosKey, cosmosKey) {
			return nil, errorsmod.Wrap(types.ErrInvalidGenesisState, "validator key pair reverse index mismatch")
		}

		keyPair := &types.ValidatorPublicKeyPair{
			MinaKey:   minaKey,
			CosmosKey: cosmosKey,
		}

		validatorKeyPairs = append(validatorKeyPairs, keyPair)

		validatorCosmosToMinaIterator.Next()
	}

	validatorMinaToCosmosIterator, err := k.validatorMinaToCosmos.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer validatorMinaToCosmosIterator.Close()

	for validatorMinaToCosmosIterator.Valid() {
		minaKey, err := validatorMinaToCosmosIterator.Key()
		if err != nil {
			return nil, err
		}
		cosmosKey, err := validatorMinaToCosmosIterator.Value()
		if err != nil {
			return nil, err
		}
		forwardMinaKey, err := k.validatorCosmosToMina.Get(ctx, cosmosKey)
		if err != nil {
			return nil, errorsmod.Wrap(types.ErrInvalidGenesisState, "validator reverse key pair missing forward index")
		}
		if !bytes.Equal(forwardMinaKey, minaKey) {
			return nil, errorsmod.Wrap(types.ErrInvalidGenesisState, "validator reverse key pair forward index mismatch")
		}

		validatorMinaToCosmosIterator.Next()
	}

	return validatorKeyPairs, nil
}
