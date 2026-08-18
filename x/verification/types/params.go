package types

import (
	"fmt"
)

const DefaultMaxProofsPerBlock uint32 = 256

func NewParams(maxProofsPerBlock uint32) Params {
	return Params{MaxProofsPerBlock: maxProofsPerBlock}
}

func DefaultParams() Params {
	return NewParams(DefaultMaxProofsPerBlock)
}

func (p Params) Validate() error {
	if p.MaxProofsPerBlock == 0 || p.MaxProofsPerBlock > MaxVoteIndexExclusive {
		return fmt.Errorf("max proofs per block must be between 1 and %d: %d", MaxVoteIndexExclusive, p.MaxProofsPerBlock)
	}

	return nil
}
