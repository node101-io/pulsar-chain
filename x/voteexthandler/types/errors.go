package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/voteexthandler module sentinel errors
var (
	ErrInvalidSigner            = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrInternal                 = errors.Register(ModuleName, 1101, "internal error")
	ErrFailedToUnmarshal        = errors.Register(ModuleName, 1102, "failed to unmarshal mina public key")
	ErrAddressMismatch          = errors.Register(ModuleName, 1103, "validator address mismatch")
	ErrFailedToGetVoteExtBody   = errors.Register(ModuleName, 1104, "failed to get vote extension body")
	ErrValidatorSetRootMismatch = errors.Register(ModuleName, 1105, "validator set root mismatch")
	ErrBlockHeightMismatch      = errors.Register(ModuleName, 1106, "block height mismatch")
	ErrStateRootMismatch        = errors.Register(ModuleName, 1107, "state root mismatch")
	ErrInvalidSigEncoding       = errors.Register(ModuleName, 1108, "invalid signature encoding")
)
