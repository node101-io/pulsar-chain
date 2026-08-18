package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

type Keeper struct {
	storeService  corestore.KVStoreService
	c             codec.Codec
	addressCodec  address.Codec
	authority     []byte
	stakingKeeper types.StakingKeeper

	Schema                 collections.Schema
	Params                 collections.Item[types.Params]
	ProofCountByHeight     collections.Map[uint64, uint32]
	PendingProofs          collections.Map[types.ProofStoreKey, types.ProofRecord]
	SeenProofHashes        collections.Map[[]byte, types.ProofKey]
	ValidatorSnapshots     collections.KeySet[types.ValidatorSnapshotStoreKey]
	ValidatorCountByHeight collections.Map[uint64, uint32]
	Commitments            collections.Map[types.CommitmentStoreKey, []byte]
	CommitmentsByHeight    collections.KeySet[types.CommitmentHeightStoreKey]
	VerificationVotes      collections.Map[types.VerificationVoteStoreKey, uint32]
	ProofTallies           collections.Map[types.ProofStoreKey, types.ProofTally]
	FinalProofResults      collections.Map[types.ProofStoreKey, types.FinalProofResult]
}

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

func (k Keeper) GetAuthority() []byte {
	return append([]byte(nil), k.authority...)
}
