package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/votepersistence module sentinel errors
var (
	ErrInvalidSigner         = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrVoteExtMarkerNotFound = errors.Register(ModuleName, 1101, "vote extension marker not found")
	ErrInvalidVoteExtension  = errors.Register(ModuleName, 1102, "invalid vote extension")
)
