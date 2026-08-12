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
	ErrInvalidPushNewProofHashRequest                    = errors.Register(ModuleName, 1103, "invalid push new proof hash request")
	ErrInvalidCreatorAddress                             = errors.Register(ModuleName, 1104, "invalid creator address")
	ErrInvalidProofHashLength                            = errors.Register(ModuleName, 1105, "proof hash must be exactly 32 bytes")
	ErrFailedToAppendPendingProof                        = errors.Register(ModuleName, 1106, "failed to append pending proof")
	ErrMaxProofRangeMustBeGreaterThanZero                = errors.Register(ModuleName, 1107, "max_proof_range must be greater than 0")
)
