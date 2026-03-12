package types

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:       DefaultParams(),
		UserKeyPairs: DefaultUserKeyPairs(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {

	for _, keyPair := range gs.UserKeyPairs {
		err := ValidateKeyPair(*keyPair)
		if err != nil {
			return err
		}
	}

	return gs.Params.Validate()
}
