package types

import "cosmossdk.io/collections"

const (
	ModuleName    = "verification"
	StoreKey      = ModuleName
	GovModuleName = "gov"
)

var (
	// Prefixes are consensus-store namespaces. SeenProofHashPrefix is
	// intentionally permanent so a proof hash can never be registered twice,
	// even after finalization. The other lifecycle state is pruned when it can no
	// longer affect a future commitment, revelation, query, or final result.
	ParamsKey                = collections.NewPrefix(0x00)
	ProofCountPrefix         = collections.NewPrefix(0x01)
	PendingProofPrefix       = collections.NewPrefix(0x02)
	SeenProofHashPrefix      = collections.NewPrefix(0x03)
	ValidatorPowerPrefix     = collections.NewPrefix(0x04)
	TotalVotingPowerPrefix   = collections.NewPrefix(0x05)
	CommitmentPrefix         = collections.NewPrefix(0x06)
	VerificationVotePrefix   = collections.NewPrefix(0x07)
	ProofTallyPrefix         = collections.NewPrefix(0x08)
	FinalProofResultPrefix   = collections.NewPrefix(0x09)
	CommitmentByHeightPrefix = collections.NewPrefix(0x0a)
)

type (
	// ProofStoreKey is the canonical absolute-height and in-block-index proof ID.
	ProofStoreKey = collections.Pair[uint64, uint32]
	// ValidatorPowerStoreKey records historical power at a proof submission height.
	ValidatorPowerStoreKey = collections.Pair[uint64, []byte]
	// CommitmentStoreKey supports validator-first commitment queries.
	CommitmentStoreKey = collections.Pair[[]byte, uint64]
	// CommitmentHeightStoreKey is the reverse index used for bounded pruning.
	CommitmentHeightStoreKey = collections.Pair[uint64, []byte]
	// VerificationVoteStoreKey stores one validator's effective vote per proof.
	VerificationVoteStoreKey = collections.Triple[uint64, uint32, []byte]
)

// NewProofStoreKey constructs the canonical proof storage key.
func NewProofStoreKey(height uint64, index uint32) ProofStoreKey {
	return collections.Join(height, index)
}

// NewValidatorPowerStoreKey constructs a proof-height validator-power key.
func NewValidatorPowerStoreKey(height uint64, validator []byte) ValidatorPowerStoreKey {
	return collections.Join(height, validator)
}

// NewCommitmentStoreKey constructs the primary validator-height commitment key.
func NewCommitmentStoreKey(validator []byte, height uint64) CommitmentStoreKey {
	return collections.Join(validator, height)
}

// NewCommitmentHeightStoreKey constructs the height-validator pruning key.
func NewCommitmentHeightStoreKey(height uint64, validator []byte) CommitmentHeightStoreKey {
	return collections.Join(height, validator)
}

// NewVerificationVoteStoreKey constructs a proof-validator vote key.
func NewVerificationVoteStoreKey(height uint64, index uint32, validator []byte) VerificationVoteStoreKey {
	return collections.Join3(height, index, validator)
}
