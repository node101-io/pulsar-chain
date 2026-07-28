package types

import (
	"strings"

	"cosmossdk.io/errors"
	minaaddress "github.com/node101-io/mina-signer-go/address"
)

// NewParams creates a new Params instance.
func NewParams(confirmationDepth int64, contractAddress string) Params {
	return Params{
		ConfirmationDepth: confirmationDepth,
		ContractAddress:   contractAddress,
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return Params{
		ConfirmationDepth: 32,
		ContractAddress:   "B62qjRDirGFRf5dvNcGzMs5oWzQ2VyNcygnoKM2MkxB9PFUp7Utdraf",
	}
}

// Validate validates the set of params.
func (p Params) Validate() error {

	if p.ConfirmationDepth <= 0 {
		return ErrConfirmationDepthMustBeGreaterThanZero
	}
	if strings.TrimSpace(p.ContractAddress) == "" {
		return ErrEmptyContractAddress
	}
	if _, err := minaaddress.NewAddress(p.ContractAddress).Marshal(); err != nil {
		return errors.Wrap(ErrInvalidContractAddress, p.ContractAddress)
	}
	return nil
}
