package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/bridge module sentinel errors
var (
	ErrInvalidSigner                  = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrUnspecified                    = errors.Register(ModuleName, 1101, "unspecified action type")
	ErrNotEnoughBalance               = errors.Register(ModuleName, 1102, "not enough balance")
	ErrBankKeeperNotConfigured        = errors.Register(ModuleName, 1103, "bank keeper is not configured")
	ErrKeyRegistryKeeperNotConfigured = errors.Register(ModuleName, 1104, "keyregistry keeper is not configured")
	ErrMinaBlockNotFinalized          = errors.Register(ModuleName, 1105, "mina block not finalized")
)
