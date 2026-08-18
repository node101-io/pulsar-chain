package types

import "cosmossdk.io/collections"

const (
	ModuleName    = "verification"
	StoreKey      = ModuleName
	GovModuleName = "gov"
)

var (
	ParamsKey                = collections.NewPrefix(0x00)
	ProofCountPrefix         = collections.NewPrefix(0x01)
	PendingProofPrefix       = collections.NewPrefix(0x02)
	SeenProofHashPrefix      = collections.NewPrefix(0x03)
	ValidatorSnapshotPrefix  = collections.NewPrefix(0x04)
	ValidatorCountPrefix     = collections.NewPrefix(0x05)
	CommitmentPrefix         = collections.NewPrefix(0x06)
	VerificationVotePrefix   = collections.NewPrefix(0x07)
	ProofTallyPrefix         = collections.NewPrefix(0x08)
	FinalProofResultPrefix   = collections.NewPrefix(0x09)
	CommitmentByHeightPrefix = collections.NewPrefix(0x0a)
)

type (
	ProofStoreKey             = collections.Pair[uint64, uint32]
	ValidatorSnapshotStoreKey = collections.Pair[uint64, []byte]
	CommitmentStoreKey        = collections.Pair[[]byte, uint64]
	CommitmentHeightStoreKey  = collections.Pair[uint64, []byte]
	VerificationVoteStoreKey  = collections.Triple[uint64, uint32, []byte]
)

func NewProofStoreKey(height uint64, index uint32) ProofStoreKey {
	return collections.Join(height, index)
}

func NewValidatorSnapshotStoreKey(height uint64, validator []byte) ValidatorSnapshotStoreKey {
	return collections.Join(height, validator)
}

func NewCommitmentStoreKey(validator []byte, height uint64) CommitmentStoreKey {
	return collections.Join(validator, height)
}

func NewCommitmentHeightStoreKey(height uint64, validator []byte) CommitmentHeightStoreKey {
	return collections.Join(height, validator)
}

func NewVerificationVoteStoreKey(height uint64, index uint32, validator []byte) VerificationVoteStoreKey {
	return collections.Join3(height, index, validator)
}
