package types

const (
	// CommitmentDeadline gives both the commitment offset and its final reveal
	// age. A commitment at C covers proof heights C-3 and C-2 and is retained
	// through C+3. Keeping one shared constant makes commitment construction,
	// reveal validation, local-journal pruning, and chain pruning agree.
	CommitmentDeadline uint64 = 3
	// RevealDeadline is the number of blocks in which a proof leaf can
	// contribute newly revealed votes. After this window, the leaf can only help
	// reconstruct a root; it cannot change the proof tally.
	RevealDeadline uint64 = 2
	// VerificationLifetime spans submission through finalization, inclusive,
	// and therefore places finalization at EndBlock(H+5).
	VerificationLifetime = 1 + CommitmentDeadline + RevealDeadline
	// MaxVoteIndexExclusive keeps indices encodable in a single byte and bounds
	// per-proof-height verification work.
	MaxVoteIndexExclusive = 256
	// MaxRevelationsPerBatch bounds one validator's per-block reveal work. One
	// revelation entry opens one commitment root, and that root always contains
	// two proof-height leaves. A batch may contain three entries because only the
	// commitments from C-1, C-2, and C-3 can still be revealable in block C.
	MaxRevelationsPerBatch = 3

	DigestSize              = 32
	ProofHashSize           = DigestSize
	PublicInputsHashSize    = DigestSize
	VerificationKeyHashSize = DigestSize
	VerificationIDSize      = DigestSize
	CommitmentHashSize      = 16
	LeafHashSize            = 16
	SaltSize                = 16
)

type LeafTiming uint8

const (
	// LeafTooEarly means the value preimage must not be disclosed yet.
	LeafTooEarly LeafTiming = iota
	// LeafActive means a revealed value contributes votes to the tally.
	LeafActive
	// LeafExpired means only the leaf hash is needed for root reconstruction.
	LeafExpired
)
