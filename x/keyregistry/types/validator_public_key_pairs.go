package types

func NewValidatorPublicKeyPairs() []*ValidatorPublicKeyPair {
	return []*ValidatorPublicKeyPair{}
}

func DefaultValidatorPublicKeyPairs() []*ValidatorPublicKeyPair {
	return NewValidatorPublicKeyPairs()
}

// Validate validates the set of params.
func (k ValidatorPublicKeyPair) Validate() error {
	if err := ValidateValidatorCosmosPublicKey(k.CosmosKey); err != nil {
		return err
	}

	if err := ValidateMinaPublicKey(k.MinaKey); err != nil {
		return err
	}

	return nil
}
