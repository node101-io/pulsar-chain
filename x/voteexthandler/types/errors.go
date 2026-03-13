package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/voteexthandler module sentinel errors
var (
	ErrInvalidSigner                          = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrInternal                               = errors.Register(ModuleName, 1101, "internal error")
	ErrFailedToUnmarshal                      = errors.Register(ModuleName, 1102, "failed to unmarshal mina public key")
	ErrAddressMismatch                        = errors.Register(ModuleName, 1103, "validator address mismatch")
	ErrFailedToGetVoteExtBody                 = errors.Register(ModuleName, 1104, "failed to get vote extension body")
	ErrValidatorSetRootMismatch               = errors.Register(ModuleName, 1105, "validator set root mismatch")
	ErrBlockHeightMismatch                    = errors.Register(ModuleName, 1106, "block height mismatch")
	ErrStateRootMismatch                      = errors.Register(ModuleName, 1107, "state root mismatch")
	ErrInvalidSigEncoding                     = errors.Register(ModuleName, 1108, "invalid signature encoding")
	ErrFailedToMarshal                        = errors.Register(ModuleName, 1109, "failed to marshall")
	ErrMissingVoteExt                         = errors.Register(ModuleName, 1110, "proposal missing VOTEEXT transaction")
	ErrMalformedVoteExtPayload                = errors.Register(ModuleName, 1111, "malformed VOTEEXT payload")
	ErrValidatorVoteExtMissing                = errors.Register(ModuleName, 1112, "validator's vote extension missing from proposal")
	ErrFailedToConvertPubKeyToAddr            = errors.Register(ModuleName, 1113, "failed to convert public key to address")
	ErrMalformedVoteExtEntry                  = errors.Register(ModuleName, 1114, "malformed vote extension entry")
	ErrSigVerificationFailed                  = errors.Register(ModuleName, 1115, "signature verification failed")
	ErrUnknownValidator                       = errors.Register(ModuleName, 1116, "unknown validator address")
	ErrFailedToSign                           = errors.Register(ModuleName, 1117, "failed to sign message")
	ErrFailedToApplyValidatorUpdates          = errors.Register(ModuleName, 1118, "failed to apply validator updates")
	ErrFailedToComputeNewSetRoot              = errors.Register(ModuleName, 1119, "failed to compute new validator set root")
	ErrFailedToComputeInitialValidatorSetRoot = errors.Register(ModuleName, 1120, "failed to compute initial validator set root")
	ErrFailedToGetKeystore                    = errors.Register(ModuleName, 1121, "failed to get key store for validator")
)
