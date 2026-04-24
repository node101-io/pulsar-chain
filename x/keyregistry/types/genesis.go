package types

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:                DefaultParams(),
		UserCosmosToMina:      DefaultUserPublicKeyPair(),
		UserMinaToCosmos:      DefaultUserPublicKeyPair(),
		ValidatorCosmosToMina: DefaultValidatorPublicKeyPair(),
		ValidatorMinaToCosmos: DefaultValidatorPublicKeyPair(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {

	for _, keyPair := range gs.UserCosmosToMina {
		if keyPair == nil {
			return ErrNilKeyPair
		}
		kp := *keyPair
		err := kp.Validate()
		if err != nil {
			return err
		}
	}

	for _, keyPair := range gs.UserMinaToCosmos {
		if keyPair == nil {
			return ErrNilKeyPair
		}
		kp := *keyPair
		err := kp.Validate()
		if err != nil {
			return err
		}
	}

	for _, keyPair := range gs.ValidatorCosmosToMina {
		if keyPair == nil {
			return ErrNilKeyPair
		}
		kp := *keyPair
		err := kp.Validate()
		if err != nil {
			return err
		}
	}
	for _, keyPair := range gs.ValidatorMinaToCosmos {
		if keyPair == nil {
			return ErrNilKeyPair
		}
		kp := *keyPair
		err := kp.Validate()
		if err != nil {
			return err
		}
	}

	return gs.Params.Validate()
}
