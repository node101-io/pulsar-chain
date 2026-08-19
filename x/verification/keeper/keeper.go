package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

// Keeper owns the replicated half of the verification protocol. It registers
// proof identities, freezes validator eligibility, accepts authenticated
// commitment and revelation actions, tracks one effective vote per validator,
// and produces immutable final results. Proof bytes, verification jobs, salts,
// and unrevealed vote lists remain validator-local and never enter these
// collections.
type Keeper struct {
	storeService  corestore.KVStoreService
	c             codec.Codec
	addressCodec  address.Codec
	authority     []byte
	stakingKeeper types.StakingKeeper

	// Proof lifecycle state uses an absolute submission height and a contiguous
	// in-block index. Absolute keys avoid ring-buffer aliases when old state is
	// pruned and heights continue to grow.
	Schema             collections.Schema
	Params             collections.Item[types.Params]
	ProofCountByHeight collections.Map[uint64, uint32]
	PendingProofs      collections.Map[types.ProofStoreKey, types.ProofRecord]
	// SeenProofHashes is permanent so a finalized and pruned hash cannot be
	// registered again.
	SeenProofHashes collections.Map[[]byte, types.ProofKey]
	// ValidatorSnapshots and ValidatorCountByHeight freeze both membership and
	// the finalization denominator at the proof submission height. Later staking
	// changes therefore cannot retroactively change who was eligible to vote.
	ValidatorSnapshots     collections.KeySet[types.ValidatorSnapshotStoreKey]
	ValidatorCountByHeight collections.Map[uint64, uint32]
	// Commitments store only opaque roots. Their salts and vote preimages stay in
	// the validator's local journal until an allowed revelation block.
	Commitments collections.Map[types.CommitmentStoreKey, []byte]
	// CommitmentsByHeight mirrors Commitments only to make height pruning
	// bounded and deterministic.
	CommitmentsByHeight collections.KeySet[types.CommitmentHeightStoreKey]
	// VerificationVotes stores the validator's effective state, including the
	// terminal EQUIVOCATED marker. ProofTallies is a derived counter kept for
	// bounded finalization work.
	VerificationVotes collections.Map[types.VerificationVoteStoreKey, uint32]
	ProofTallies      collections.Map[types.ProofStoreKey, types.ProofTally]
	// FinalProofResults replaces pruned pending state and is never mutated after
	// EndBlock(H+5).
	FinalProofResults collections.Map[types.ProofStoreKey, types.FinalProofResult]
}

// NewKeeper builds the verification collections schema and validates its
// mandatory authority and staking dependencies.
func NewKeeper(
	storeService corestore.KVStoreService,
	c codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	stakingKeeper types.StakingKeeper,
) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}
	if stakingKeeper == nil {
		panic("verification keeper requires a staking keeper")
	}

	sb := collections.NewSchemaBuilder(storeService)
	proofKeyCodec := collections.PairKeyCodec(collections.Uint64Key, collections.Uint32Key)
	validatorSnapshotKeyCodec := collections.PairKeyCodec(collections.Uint64Key, collections.BytesKey)
	commitmentKeyCodec := collections.PairKeyCodec(collections.BytesKey, collections.Uint64Key)
	commitmentHeightKeyCodec := collections.PairKeyCodec(collections.Uint64Key, collections.BytesKey)
	voteKeyCodec := collections.TripleKeyCodec(collections.Uint64Key, collections.Uint32Key, collections.BytesKey)

	k := Keeper{
		storeService:  storeService,
		c:             c,
		addressCodec:  addressCodec,
		authority:     append([]byte(nil), authority...),
		stakingKeeper: stakingKeeper,

		Params:                 collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](c)),
		ProofCountByHeight:     collections.NewMap(sb, types.ProofCountPrefix, "proof_count_by_height", collections.Uint64Key, collections.Uint32Value),
		PendingProofs:          collections.NewMap(sb, types.PendingProofPrefix, "pending_proofs", proofKeyCodec, codec.CollValue[types.ProofRecord](c)),
		SeenProofHashes:        collections.NewMap(sb, types.SeenProofHashPrefix, "seen_proof_hashes", collections.BytesKey, codec.CollValue[types.ProofKey](c)),
		ValidatorSnapshots:     collections.NewKeySet(sb, types.ValidatorSnapshotPrefix, "validator_snapshots", validatorSnapshotKeyCodec),
		ValidatorCountByHeight: collections.NewMap(sb, types.ValidatorCountPrefix, "validator_count_by_height", collections.Uint64Key, collections.Uint32Value),
		Commitments:            collections.NewMap(sb, types.CommitmentPrefix, "commitments", commitmentKeyCodec, collections.BytesValue),
		CommitmentsByHeight:    collections.NewKeySet(sb, types.CommitmentByHeightPrefix, "commitments_by_height", commitmentHeightKeyCodec),
		VerificationVotes:      collections.NewMap(sb, types.VerificationVotePrefix, "verification_votes", voteKeyCodec, collections.Uint32Value),
		ProofTallies:           collections.NewMap(sb, types.ProofTallyPrefix, "proof_tallies", proofKeyCodec, codec.CollValue[types.ProofTally](c)),
		FinalProofResults:      collections.NewMap(sb, types.FinalProofResultPrefix, "final_proof_results", proofKeyCodec, codec.CollValue[types.FinalProofResult](c)),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

// GetAuthority returns a copy of the governance authority address.
func (k Keeper) GetAuthority() []byte {
	return append([]byte(nil), k.authority...)
}
