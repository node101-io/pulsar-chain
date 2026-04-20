package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/keyregistry module sentinel errors
var (
	ErrInvalidSigner               = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrInvalidSignature            = errors.Register(ModuleName, 1101, "invalid signature")
	ErrUserSecondaryKeyExists      = errors.Register(ModuleName, 1102, "user's secondary key already exists")
	ErrValidatorSecondaryKeyExists = errors.Register(ModuleName, 1103, "validator's secondary key already exists")
	ErrInvalidCreatorAddres        = errors.Register(ModuleName, 1104, "invalid creator address")
	ErrInvalidPublicKey            = errors.Register(ModuleName, 1105, "invalid public key")
	ErrUserNotRegistered           = errors.Register(ModuleName, 1106, "user has not been registered")
	ErrValidatorNotRegistered      = errors.Register(ModuleName, 1107, "validator has not been registered")
)
