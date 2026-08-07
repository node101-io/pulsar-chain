package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/verification module sentinel errors
var (
	ErrInvalidSigner                                     = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrPendingProofAlreadyExists                         = errors.Register(ModuleName, 1101, "pending proof already exists")
	ErrPendingProofBlocksWindowSizeMustBeGreaterThanZero = errors.Register(ModuleName, 1102, "pending_proof_blocks_window_size must be greater than 0")
)
