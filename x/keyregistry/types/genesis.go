package types

import errorsmod "cosmossdk.io/errors"

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:            DefaultParams(),
		UserKeyPairs:      DefaultUserPublicKeyPair(),
		ValidatorKeyPairs: DefaultValidatorPublicKeyPair(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := validateUserGenesisKeyPairs(gs.UserKeyPairs); err != nil {
		return err
	}

	if err := validateValidatorGenesisKeyPairs(gs.ValidatorKeyPairs); err != nil {
		return err
	}

	return gs.Params.Validate()
}

func validateUserGenesisKeyPairs(keyPairs []*UserPublicKeyPair) error {
	seenCosmosKeys := make(map[string]struct{}, len(keyPairs))
	seenMinaKeys := make(map[string]struct{}, len(keyPairs))

	for _, keyPair := range keyPairs {
		if keyPair == nil {
			return ErrNilKeyPair
		}

		if err := keyPair.Validate(); err != nil {
			return err
		}

		cosmosKey := string(keyPair.CosmosKey)
		if _, exists := seenCosmosKeys[cosmosKey]; exists {
			return errorsmod.Wrap(ErrInvalidGenesisState, "duplicate user cosmos key")
		}
		seenCosmosKeys[cosmosKey] = struct{}{}

		minaKey := string(keyPair.MinaKey)
		if _, exists := seenMinaKeys[minaKey]; exists {
			return errorsmod.Wrap(ErrInvalidGenesisState, "duplicate user mina key")
		}
		seenMinaKeys[minaKey] = struct{}{}
	}

	return nil
}

func validateValidatorGenesisKeyPairs(keyPairs []*ValidatorPublicKeyPair) error {
	seenConsensusKeys := make(map[string]struct{}, len(keyPairs))
	seenMinaKeys := make(map[string]struct{}, len(keyPairs))

	for _, keyPair := range keyPairs {
		if keyPair == nil {
			return ErrNilKeyPair
		}

		if err := keyPair.Validate(); err != nil {
			return err
		}

		consensusKey := string(keyPair.CosmosKey)
		if _, exists := seenConsensusKeys[consensusKey]; exists {
			return errorsmod.Wrap(ErrInvalidGenesisState, "duplicate validator consensus key")
		}
		seenConsensusKeys[consensusKey] = struct{}{}

		minaKey := string(keyPair.MinaKey)
		if _, exists := seenMinaKeys[minaKey]; exists {
			return errorsmod.Wrap(ErrInvalidGenesisState, "duplicate validator mina key")
		}
		seenMinaKeys[minaKey] = struct{}{}
	}

	return nil
}
