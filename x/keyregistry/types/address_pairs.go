package types

import (
	"cosmossdk.io/errors"
)

const CosmosAddrLen = 20
const MinaAddrLen = 45

func NewAddressPairs() []*AddressPair {
	return []*AddressPair{}
}
func DefaultUserAddressPairs() []*AddressPair {
	return NewAddressPairs()
}

func DefaultValidatorKeyPairs() []*AddressPair {
	return NewAddressPairs()
}

func ValidateAddressPair(k AddressPair) error {
	if len(k.CosmosAddr) != CosmosAddrLen {
		return errors.Wrap(ErrInvalidAddress, "cosmos address must be valid (20 bytes)")
	}

	if len(k.MinaAddr) != MinaAddrLen {
		return errors.Wrap(ErrInvalidAddress, "mina address must be valid")
	}
	return nil
}

// Validate validates the set of params.
func (k AddressPair) Validate() error {
	return ValidateAddressPair(k)
}
