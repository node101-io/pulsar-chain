package types

import (
	"strings"

	"cosmossdk.io/errors"
	minaaddress "github.com/node101-io/mina-signer-go/address"
)

const (
	defaultStartBlockHeight int64 = 1
	defaultMaxBlockRange    int64 = 1000
)

// NewParams creates a new Params instance.
func NewParams(
	confirmationDepth int64,
	contractAddress string,
	startBlockHeight int64,
	maxBlockRange int64,
) Params {
	return Params{
		ConfirmationDepth: confirmationDepth,
		ContractAddress:   contractAddress,
		StartBlockHeight:  startBlockHeight,
		MaxBlockRange:     maxBlockRange,
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return Params{}
}

// DefaultTestParams returns valid bridge params for tests and simulation.
func DefaultTestParams() Params {
	return Params{
		ConfirmationDepth: 32,
		ContractAddress:   "B62qjRDirGFRf5dvNcGzMs5oWzQ2VyNcygnoKM2MkxB9PFUp7Utdraf",
		StartBlockHeight:  defaultStartBlockHeight,
		MaxBlockRange:     defaultMaxBlockRange,
	}
}

// Validate validates the set of params.
func (p Params) Validate() error {

	if p.ConfirmationDepth <= 0 {
		return ErrConfirmationDepthMustBeGreaterThanZero
	}
	if p.StartBlockHeight <= 0 {
		return ErrStartBlockHeightMustBeGreaterThanZero
	}
	if p.MaxBlockRange <= 0 {
		return ErrMaxBlockRangeMustBeGreaterThanZero
	}
	if strings.TrimSpace(p.ContractAddress) == "" {
		return ErrEmptyContractAddress
	}
	if _, err := minaaddress.NewAddress(p.ContractAddress).Marshal(); err != nil {
		return errors.Wrap(ErrInvalidContractAddress, p.ContractAddress)
	}

	return nil
}
