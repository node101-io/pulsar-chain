package types

const (
	defaultPendingProofsWindowSize int64 = 6
	defaultMaxProofRange           int64 = 256
)

// NewParams creates a new Params instance.
func NewParams(pendingProofsWindowSize, maxProofRange int64) Params {
	return Params{
		PendingProofBlocksWindowSize: pendingProofsWindowSize,
		MaxProofRange:                maxProofRange,
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams(defaultPendingProofsWindowSize, defaultMaxProofRange)
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if p.PendingProofBlocksWindowSize <= 0 {
		return ErrPendingProofBlocksWindowSizeMustBeGreaterThanZero
	}
	if p.MaxProofRange <= 0 {
		return ErrMaxProofRangeMustBeGreaterThanZero
	}
	return nil
}
