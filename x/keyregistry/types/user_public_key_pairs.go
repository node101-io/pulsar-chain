package types

func NewUserPublicKeyPairs() []*UserPublicKeyPair {
	return []*UserPublicKeyPair{}
}

func DefaultUserPublicKeyPairs() []*UserPublicKeyPair {
	return NewUserPublicKeyPairs()
}

// Validate validates the set of params.
func (k UserPublicKeyPair) Validate() error {
	if err := ValidateUserCosmosPublicKey(k.CosmosKey); err != nil {
		return err
	}

	if err := ValidateMinaPublicKey(k.MinaKey); err != nil {
		return err
	}

	return nil
}
