package keeper

import (
	"bytes"
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

// Keeper owns the replicated half of the verification protocol. It registers
// proof identities, freezes historical validator power, accepts authenticated
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
	// SeenVerificationIDs is permanent so the same complete verification request
	// cannot be registered again after its lifecycle state has been pruned.
	SeenVerificationIDs collections.Map[[]byte, types.ProofKey]
	// ValidatorPowers and TotalVotingPowerByHeight materialize the immutable
	// HistoricalInfo for proof heights. Later staking changes therefore cannot
	// change either voter eligibility or the finalization denominator.
	ValidatorPowers          collections.Map[types.ValidatorPowerStoreKey, int64]
	TotalVotingPowerByHeight collections.Map[uint64, int64]
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
	validatorPowerKeyCodec := collections.PairKeyCodec(collections.Uint64Key, collections.BytesKey)
	commitmentKeyCodec := collections.PairKeyCodec(collections.BytesKey, collections.Uint64Key)
	commitmentHeightKeyCodec := collections.PairKeyCodec(collections.Uint64Key, collections.BytesKey)
	voteKeyCodec := collections.TripleKeyCodec(collections.Uint64Key, collections.Uint32Key, collections.BytesKey)

	k := Keeper{
		storeService:  storeService,
		c:             c,
		addressCodec:  addressCodec,
		authority:     append([]byte(nil), authority...),
		stakingKeeper: stakingKeeper,

		Params:                   collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](c)),
		ProofCountByHeight:       collections.NewMap(sb, types.ProofCountPrefix, "proof_count_by_height", collections.Uint64Key, collections.Uint32Value),
		PendingProofs:            collections.NewMap(sb, types.PendingProofPrefix, "pending_proofs", proofKeyCodec, codec.CollValue[types.ProofRecord](c)),
		SeenVerificationIDs:      collections.NewMap(sb, types.SeenVerificationIDPrefix, "seen_verification_ids", collections.BytesKey, codec.CollValue[types.ProofKey](c)),
		ValidatorPowers:          collections.NewMap(sb, types.ValidatorPowerPrefix, "validator_powers", validatorPowerKeyCodec, collections.Int64Value),
		TotalVotingPowerByHeight: collections.NewMap(sb, types.TotalVotingPowerPrefix, "total_voting_power_by_height", collections.Uint64Key, collections.Int64Value),
		Commitments:              collections.NewMap(sb, types.CommitmentPrefix, "commitments", commitmentKeyCodec, collections.BytesValue),
		CommitmentsByHeight:      collections.NewKeySet(sb, types.CommitmentByHeightPrefix, "commitments_by_height", commitmentHeightKeyCodec),
		VerificationVotes:        collections.NewMap(sb, types.VerificationVotePrefix, "verification_votes", voteKeyCodec, collections.Uint32Value),
		ProofTallies:             collections.NewMap(sb, types.ProofTallyPrefix, "proof_tallies", proofKeyCodec, codec.CollValue[types.ProofTally](c)),
		FinalProofResults:        collections.NewMap(sb, types.FinalProofResultPrefix, "final_proof_results", proofKeyCodec, codec.CollValue[types.FinalProofResult](c)),
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

func (k Keeper) FinalProofResultByProofHash(
	ctx context.Context,
	proofHash []byte,
) (types.FinalProofResult, error) {
	if len(proofHash) != types.ProofHashSize {
		return types.FinalProofResult{}, types.ErrInvalidProofHash
	}

	iterator, err := k.FinalProofResults.Iterate(ctx, nil)
	if err != nil {
		return types.FinalProofResult{}, err
	}
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		result, err := iterator.Value()
		if err != nil {
			return types.FinalProofResult{}, err
		}

		if bytes.Equal(result.ProofHash, proofHash) {
			return result, nil
		}
	}

	return types.FinalProofResult{}, types.ErrProofNotFound
}
