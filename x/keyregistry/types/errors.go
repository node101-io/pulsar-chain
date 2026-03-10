package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/keyregistry module sentinel errors
var (
	ErrInvalidSigner        = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrInvalidSignature     = errors.Register(ModuleName, 1101, "invalid signature")
	ErrSecondaryKeyExists   = errors.Register(ModuleName, 1102, "secondary key already exists")
	ErrInvalidCreatorAddres = errors.Register(ModuleName, 1103, "invalid creator address")
	ErrInvalidPublicKey     = errors.Register(ModuleName, 1104, "invalid public key")
)
