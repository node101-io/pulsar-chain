package types

const (
	CommitmentDeadline     uint64 = 3
	RevealDeadline         uint64 = 2
	VerificationLifetime          = 1 + CommitmentDeadline + RevealDeadline
	MaxVoteIndexExclusive         = 256
	MaxRevelationsPerBatch        = 3

	ProofHashSize      = 32
	CommitmentHashSize = 16
	LeafHashSize       = 16
	SaltSize           = 16
)

type LeafTiming uint8

const (
	LeafTooEarly LeafTiming = iota
	LeafActive
	LeafExpired
)
