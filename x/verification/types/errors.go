package types

import errorsmod "cosmossdk.io/errors"

var (
	// Error codes are stable module API identifiers. Keep existing numeric
	// values unchanged when adding new errors.
	ErrInvalidSigner                 = errorsmod.Register(ModuleName, 1100, "invalid signer")
	ErrInvalidProofHash              = errorsmod.Register(ModuleName, 1101, "invalid proof hash")
	ErrDuplicateVerificationRequest  = errorsmod.Register(ModuleName, 1102, "verification request already submitted")
	ErrMaxProofsPerBlock             = errorsmod.Register(ModuleName, 1103, "maximum proofs per block reached")
	ErrProofNotFound                 = errorsmod.Register(ModuleName, 1104, "proof not found")
	ErrProofHeightNotFound           = errorsmod.Register(ModuleName, 1105, "proof height not found")
	ErrEmptyValidatorSet             = errorsmod.Register(ModuleName, 1106, "validator set is empty")
	ErrInvalidValidator              = errorsmod.Register(ModuleName, 1107, "invalid validator")
	ErrInvalidCommitmentLength       = errorsmod.Register(ModuleName, 1108, "invalid commitment length")
	ErrInvalidCommitmentHeight       = errorsmod.Register(ModuleName, 1109, "invalid commitment height")
	ErrCommitmentAlreadyExists       = errorsmod.Register(ModuleName, 1110, "commitment already exists")
	ErrCommitmentNotFound            = errorsmod.Register(ModuleName, 1111, "commitment not found")
	ErrCommitmentMismatch            = errorsmod.Register(ModuleName, 1112, "commitment mismatch")
	ErrInvalidRevelationCount        = errorsmod.Register(ModuleName, 1113, "invalid revelation count")
	ErrDuplicateCommitmentRevelation = errorsmod.Register(ModuleName, 1114, "duplicate commitment revelation")
	ErrInvalidLeafRevealMode         = errorsmod.Register(ModuleName, 1115, "invalid leaf reveal mode")
	ErrInvalidLeafHashLength         = errorsmod.Register(ModuleName, 1116, "invalid leaf hash length")
	ErrInvalidSaltLength             = errorsmod.Register(ModuleName, 1117, "invalid salt length")
	ErrEarlyReveal                   = errorsmod.Register(ModuleName, 1118, "leaf value revealed too early")
	ErrUselessRevelation             = errorsmod.Register(ModuleName, 1119, "revelation contains no value leaf")
	ErrTooManyVotes                  = errorsmod.Register(ModuleName, 1120, "too many votes")
	ErrInvalidVoteIndex              = errorsmod.Register(ModuleName, 1121, "invalid vote index")
	ErrDuplicateVoteIndex            = errorsmod.Register(ModuleName, 1122, "duplicate vote index")
	ErrNonCanonicalVoteOrdering      = errorsmod.Register(ModuleName, 1123, "non-canonical vote ordering")
	ErrProofStateCorrupted           = errorsmod.Register(ModuleName, 1124, "proof state is corrupted")
	ErrInvalidProofType              = errorsmod.Register(ModuleName, 1125, "invalid proof type")
	ErrInvalidPublicInputsHash       = errorsmod.Register(ModuleName, 1126, "invalid public inputs hash")
	ErrInvalidVerificationKeyHash    = errorsmod.Register(ModuleName, 1127, "invalid verification key hash")
	ErrInvalidVerificationID         = errorsmod.Register(ModuleName, 1128, "invalid verification ID")
)
