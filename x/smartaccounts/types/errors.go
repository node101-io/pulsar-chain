package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/smartaccounts module sentinel errors
var (
	ErrInvalidSigner           = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrNilPublicKey            = errors.Register(ModuleName, 1101, "nil public key")
	ErrPublicKeyInvalidLength  = errors.Register(ModuleName, 1102, "public key invalid length")
	ErrInvalidExpirationHeight = errors.Register(ModuleName, 1103, "invalid expiration height")
	ErrNilIdentity             = errors.Register(ModuleName, 1105, "nil identity")
	ErrSessionKeyAlreadyExists = errors.Register(ModuleName, 1106, "session key already exists")
	ErrIdentityInvalidLength   = errors.Register(ModuleName, 1107, "identity invalid length")
	ErrNilPublicKeyInputs      = errors.Register(ModuleName, 1108, "nil public key inputs")
)
