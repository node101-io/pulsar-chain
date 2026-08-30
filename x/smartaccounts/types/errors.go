package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/smartaccounts module sentinel errors
var (
	ErrInvalidSigner               = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrNilPublicKey                = errors.Register(ModuleName, 1101, "nil public key")
	ErrPublicKeyInvalidLength      = errors.Register(ModuleName, 1102, "public key invalid length")
	ErrInvalidExpirationHeight     = errors.Register(ModuleName, 1103, "invalid expiration height")
	ErrNilIdentity                 = errors.Register(ModuleName, 1104, "nil identity")
	ErrSessionKeyAlreadyExists     = errors.Register(ModuleName, 1105, "session key already exists")
	ErrIdentityInvalidLength       = errors.Register(ModuleName, 1106, "identity invalid length")
	ErrNilPublicKeyInputs          = errors.Register(ModuleName, 1107, "nil public key inputs")
	ErrNilVerificationID           = errors.Register(ModuleName, 1108, "nil verification ID")
	ErrVerificationIDInvalidLength = errors.Register(ModuleName, 1109, "verification ID invalid length")
	ErrProofNotValid               = errors.Register(ModuleName, 1110, "proof is not valid")
	ErrVerificationKeyMismatch     = errors.Register(ModuleName, 1111, "verification key hash mismatch")
	ErrInvalidPublicInputsHash     = errors.Register(ModuleName, 1112, "invalid public inputs hash")
	ErrNilAccountAddress           = errors.Register(ModuleName, 1113, "nil account address")
	ErrInvalidAccountAddress       = errors.Register(ModuleName, 1114, "invalid account address")
	ErrAccountAddressMismatch      = errors.Register(ModuleName, 1115, "account address mismatch")
)
