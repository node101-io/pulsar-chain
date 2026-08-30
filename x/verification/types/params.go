package types

import (
	"fmt"
)

// DefaultMaxProofsPerBlock matches the one-byte in-block proof index space.
const DefaultMaxProofsPerBlock uint32 = 256

// NewParams constructs verification module parameters.
func NewParams(maxProofsPerBlock uint32) Params {
	return Params{MaxProofsPerBlock: maxProofsPerBlock}
}

// DefaultParams returns the production verification limits.
func DefaultParams() Params {
	return NewParams(DefaultMaxProofsPerBlock)
}

// Validate keeps the configured proof capacity within the canonical encoding.
func (p Params) Validate() error {
	if p.MaxProofsPerBlock == 0 || p.MaxProofsPerBlock > MaxVoteIndexExclusive {
		return fmt.Errorf("max proofs per block must be between 1 and %d: %d", MaxVoteIndexExclusive, p.MaxProofsPerBlock)
	}

	return nil
}
