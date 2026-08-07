package types

const defaultPendingProofsWindowSize int64 = 6

// NewParams creates a new Params instance.
func NewParams(pendingProofsWindowSize int64) Params {
	return Params{
		PendingProofBlocksWindowSize: pendingProofsWindowSize,
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams(defaultPendingProofsWindowSize)
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if p.PendingProofBlocksWindowSize <= 0 {
		return ErrPendingProofBlocksWindowSizeMustBeGreaterThanZero
	}
	return nil
}
