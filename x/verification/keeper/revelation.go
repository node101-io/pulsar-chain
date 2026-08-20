package keeper

import (
	"bytes"
	"context"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

// PlannedLeafVotes is a validated value leaf whose votes are still inside the
// proof's active reveal window. Keeping only effective leaves in the plan makes
// expired-value handling explicit: expired data can authenticate a root but
// cannot change a tally.
type PlannedLeafVotes struct {
	ProofHeight uint64
	Votes       []types.ProofVote
}

// RevelationPlan contains the effective state changes derived from one root.
// Root reconstruction has already succeeded when this value is returned, so
// later batch logic can focus on vote identity and equivocation.
type RevelationPlan struct {
	Validator        []byte
	CommitmentHeight uint64
	EffectiveLeaves  []PlannedLeafVotes
}

type proofID struct {
	height uint64
	index  uint32
}

type voteID struct {
	proof     proofID
	validator string
}

type plannedEvent struct {
	equivocation     bool
	validator        []byte
	commitmentHeight uint64
	proof            proofID
}

// ValidatedRevelationBatch stages all vote, tally, and event changes for an
// entire payload. Nothing is written while the batch is being validated. This
// gives revelation processing transaction-like behavior even when several
// roots and hundreds of votes are included together.
type ValidatedRevelationBatch struct {
	votes      map[voteID]types.VoteState
	voteOrder  []voteID
	tallies    map[proofID]types.ProofTally
	tallyOrder []proofID
	events     []plannedEvent
}

// ValidateCommitmentRevelation reconstructs a stored root and extracts only
// votes whose proof leaves are currently active. Expired leaves can still be
// supplied as values, but they do not count. This allows a validator to reveal
// the still-active half of a two-height commitment without late votes from the
// sibling half changing an already closed proof window.
func (k Keeper) ValidateCommitmentRevelation(
	ctx context.Context,
	validator []byte,
	currentHeight uint64,
	revelation types.CommitmentRevelation,
) (RevelationPlan, error) {
	plan := RevelationPlan{
		Validator:        append([]byte(nil), validator...),
		CommitmentHeight: revelation.CommitmentHeight,
	}
	if err := types.ValidateRevelationCommitmentHeight(currentHeight, revelation.CommitmentHeight); err != nil {
		return plan, err
	}
	commitmentKey := types.NewCommitmentStoreKey(validator, revelation.CommitmentHeight)
	exists, err := k.Commitments.Has(ctx, commitmentKey)
	if err != nil {
		return plan, err
	}
	if !exists {
		return plan, types.ErrCommitmentNotFound
	}
	stored, err := k.Commitments.Get(ctx, commitmentKey)
	if err != nil {
		return plan, err
	}
	if len(stored) != types.CommitmentHashSize {
		return plan, types.ErrProofStateCorrupted
	}

	leftHeight, rightHeight, err := types.CommitmentProofHeights(revelation.CommitmentHeight)
	if err != nil {
		return plan, err
	}
	// Both leaf hashes are always needed to authenticate the root, even when
	// only one leaf is open as a value. A hash-only sibling hides its salt and
	// votes while still proving that the opened value belongs to the stored root.
	leftHash, leftVotes, leftCounts, err := processLeafRevelation(currentHeight, leftHeight, revelation.Left)
	if err != nil {
		return plan, err
	}
	rightHash, rightVotes, rightCounts, err := processLeafRevelation(currentHeight, rightHeight, revelation.Right)
	if err != nil {
		return plan, err
	}
	if revelation.Left.Mode == types.LeafRevealMode_LEAF_REVEAL_MODE_HASH_ONLY &&
		revelation.Right.Mode == types.LeafRevealMode_LEAF_REVEAL_MODE_HASH_ONLY {
		return plan, types.ErrUselessRevelation
	}
	root := types.ComputeCommitmentRoot(leftHash, rightHash)
	if !bytes.Equal(root[:], stored) {
		return plan, types.ErrCommitmentMismatch
	}

	if leftCounts && len(leftVotes) > 0 {
		plan.EffectiveLeaves = append(plan.EffectiveLeaves, PlannedLeafVotes{ProofHeight: leftHeight, Votes: leftVotes})
	}
	if rightCounts && len(rightVotes) > 0 {
		plan.EffectiveLeaves = append(plan.EffectiveLeaves, PlannedLeafVotes{ProofHeight: rightHeight, Votes: rightVotes})
	}

	return plan, nil
}

func processLeafRevelation(
	currentHeight uint64,
	proofHeight uint64,
	revelation types.LeafRevelation,
) ([types.LeafHashSize]byte, []types.ProofVote, bool, error) {
	var empty [types.LeafHashSize]byte
	switch revelation.Mode {
	case types.LeafRevealMode_LEAF_REVEAL_MODE_HASH_ONLY:
		// Hash-only preserves the unopened half of the Merkle-style root without
		// exposing a salt or creating votes. A batch with two hash-only leaves is
		// rejected because it proves nothing new and only consumes block space.
		leafHash := revelation.GetLeafHash()
		if len(leafHash) != types.LeafHashSize {
			return empty, nil, false, types.ErrInvalidLeafHashLength
		}
		var out [types.LeafHashSize]byte
		copy(out[:], leafHash)
		return out, nil, false, nil

	case types.LeafRevealMode_LEAF_REVEAL_MODE_VALUE:
		if types.GetLeafTiming(currentHeight, proofHeight) == types.LeafTooEarly {
			return empty, nil, false, types.ErrEarlyReveal
		}
		value := revelation.GetValue()
		if value == nil {
			return empty, nil, false, types.ErrInvalidLeafRevealMode
		}
		if len(value.Salt) != types.SaltSize {
			return empty, nil, false, types.ErrInvalidSaltLength
		}
		if err := types.ValidateCanonicalVotes(value.Votes); err != nil {
			return empty, nil, false, err
		}
		leafHash, err := types.ComputeLeafHash(value.Salt, value.Votes)
		if err != nil {
			return empty, nil, false, err
		}
		// A value can authenticate an expired leaf, but its votes count only
		// during the leaf's active window.
		votes := append([]types.ProofVote(nil), value.Votes...)
		return leafHash, votes, types.GetLeafTiming(currentHeight, proofHeight) == types.LeafActive, nil

	default:
		return empty, nil, false, types.ErrInvalidLeafRevealMode
	}
}

// ValidateRevelationPlans performs full-batch prevalidation. Staging across
// plans is important because two revelations in one payload can touch the same
// proof and expose equivocation before any consensus state is written. Reading
// staged values first also makes duplicate votes idempotent within the batch,
// exactly as they are across different blocks.
func (k Keeper) ValidateRevelationPlans(ctx context.Context, plans []RevelationPlan) (ValidatedRevelationBatch, error) {
	batch := ValidatedRevelationBatch{
		votes:   make(map[voteID]types.VoteState),
		tallies: make(map[proofID]types.ProofTally),
	}
	for _, plan := range plans {
		for _, leaf := range plan.EffectiveLeaves {
			validatorPower, totalPower, err := k.validateVotes(ctx, plan.Validator, leaf.ProofHeight, leaf.Votes)
			if err != nil {
				return ValidatedRevelationBatch{}, err
			}
			for _, vote := range leaf.Votes {
				if err := k.stageVote(ctx, &batch, plan.Validator, leaf.ProofHeight, vote, validatorPower, totalPower); err != nil {
					return ValidatedRevelationBatch{}, err
				}
			}
		}
		batch.events = append(batch.events, plannedEvent{
			validator:        append([]byte(nil), plan.Validator...),
			commitmentHeight: plan.CommitmentHeight,
		})
	}

	return batch, nil
}

// validateVotes loads historical power once for an effective leaf and checks
// every referenced proof before any of its votes are staged.
func (k Keeper) validateVotes(ctx context.Context, validator []byte, proofHeight uint64, votes []types.ProofVote) (int64, int64, error) {
	validatorPower, totalPower, err := k.ValidatorPowerAtHeight(ctx, proofHeight, validator)
	if err != nil {
		return 0, 0, err
	}
	found, err := k.ProofCountByHeight.Has(ctx, proofHeight)
	if err != nil {
		return 0, 0, err
	}
	if !found {
		return 0, 0, types.ErrProofHeightNotFound
	}
	proofCount, err := k.ProofCountByHeight.Get(ctx, proofHeight)
	if err != nil {
		return 0, 0, err
	}
	for _, vote := range votes {
		if vote.IndexInBlock >= proofCount {
			return 0, 0, types.ErrProofNotFound
		}
		exists, err := k.PendingProofs.Has(ctx, types.NewProofStoreKey(proofHeight, vote.IndexInBlock))
		if err != nil {
			return 0, 0, err
		}
		if !exists {
			return 0, 0, types.ErrProofNotFound
		}
	}

	return validatorPower, totalPower, nil
}

func (k Keeper) stageVote(
	ctx context.Context,
	batch *ValidatedRevelationBatch,
	validator []byte,
	proofHeight uint64,
	vote types.ProofVote,
	validatorPower int64,
	totalPower int64,
) error {
	proof := proofID{height: proofHeight, index: vote.IndexInBlock}
	key := voteID{proof: proof, validator: string(validator)}
	existing, ok := batch.votes[key]
	if !ok {
		stored, err := k.loadVote(ctx, key)
		if err != nil {
			return err
		}
		existing = stored
	}
	tally, ok := batch.tallies[proof]
	if !ok {
		stored, err := k.ProofTallies.Get(ctx, types.NewProofStoreKey(proof.height, proof.index))
		if err != nil {
			return errorsmod.Wrap(types.ErrProofStateCorrupted, "missing proof tally")
		}
		tally = stored
	}
	if err := validatePowerTally(tally, totalPower); err != nil {
		return err
	}

	// A validator has at most one effective contribution. Repeating the same
	// vote is idempotent; a conflicting vote removes the old tally contribution
	// and permanently marks the validator as equivocated for this proof. The new
	// conflicting value is not added either, so equivocation can never increase
	// the chance that either side reaches threshold.
	incoming := types.VoteState_VOTE_STATE_FALSE
	if vote.Result {
		incoming = types.VoteState_VOTE_STATE_TRUE
	}
	newState := existing
	changed := false
	switch existing {
	case types.VoteState_VOTE_STATE_NONE:
		newState = incoming
		changed = true
		if incoming == types.VoteState_VOTE_STATE_TRUE {
			if validatorPower > totalPower-tally.ValidVotingPower {
				return types.ErrProofStateCorrupted
			}
			tally.ValidVotingPower += validatorPower
		} else {
			if validatorPower > totalPower-tally.InvalidVotingPower {
				return types.ErrProofStateCorrupted
			}
			tally.InvalidVotingPower += validatorPower
		}
	case types.VoteState_VOTE_STATE_TRUE:
		if incoming != existing {
			if tally.ValidVotingPower < validatorPower {
				return types.ErrProofStateCorrupted
			}
			tally.ValidVotingPower -= validatorPower
			newState = types.VoteState_VOTE_STATE_EQUIVOCATED
			changed = true
			batch.events = append(batch.events, plannedEvent{equivocation: true, validator: append([]byte(nil), validator...), proof: proof})
		}
	case types.VoteState_VOTE_STATE_FALSE:
		if incoming != existing {
			if tally.InvalidVotingPower < validatorPower {
				return types.ErrProofStateCorrupted
			}
			tally.InvalidVotingPower -= validatorPower
			newState = types.VoteState_VOTE_STATE_EQUIVOCATED
			changed = true
			batch.events = append(batch.events, plannedEvent{equivocation: true, validator: append([]byte(nil), validator...), proof: proof})
		}
	case types.VoteState_VOTE_STATE_EQUIVOCATED:
		return nil
	default:
		return types.ErrProofStateCorrupted
	}
	if err := validatePowerTally(tally, totalPower); err != nil {
		return err
	}

	if changed {
		if _, seen := batch.votes[key]; !seen {
			batch.voteOrder = append(batch.voteOrder, key)
		}
		if _, seen := batch.tallies[proof]; !seen {
			batch.tallyOrder = append(batch.tallyOrder, proof)
		}
		batch.votes[key] = newState
		batch.tallies[proof] = tally
	}

	return nil
}

func validatePowerTally(tally types.ProofTally, totalPower int64) error {
	if !types.IsValidTotalVotingPower(totalPower) ||
		tally.ValidVotingPower < 0 || tally.InvalidVotingPower < 0 ||
		tally.ValidVotingPower > totalPower-tally.InvalidVotingPower {
		return types.ErrProofStateCorrupted
	}
	return nil
}

func (k Keeper) loadVote(ctx context.Context, key voteID) (types.VoteState, error) {
	storeKey := types.NewVerificationVoteStoreKey(key.proof.height, key.proof.index, []byte(key.validator))
	exists, err := k.VerificationVotes.Has(ctx, storeKey)
	if err != nil || !exists {
		return types.VoteState_VOTE_STATE_NONE, err
	}
	value, err := k.VerificationVotes.Get(ctx, storeKey)
	if err != nil {
		return types.VoteState_VOTE_STATE_NONE, err
	}
	state := types.VoteState(value)
	if state < types.VoteState_VOTE_STATE_NONE || state > types.VoteState_VOTE_STATE_EQUIVOCATED {
		return types.VoteState_VOTE_STATE_NONE, types.ErrProofStateCorrupted
	}

	return state, nil
}

// ApplyRevelationPlans persists a previously validated batch in deterministic
// first-touch order, then emits the corresponding consensus events. The order
// does not change the mathematical result, but deterministic writes and events
// make state traces and debugging consistent across validators.
func (k Keeper) ApplyRevelationPlans(ctx context.Context, batch ValidatedRevelationBatch) error {
	for _, key := range batch.voteOrder {
		state := batch.votes[key]
		if err := k.VerificationVotes.Set(ctx,
			types.NewVerificationVoteStoreKey(key.proof.height, key.proof.index, []byte(key.validator)),
			uint32(state),
		); err != nil {
			return err
		}
	}
	for _, proof := range batch.tallyOrder {
		if err := k.ProofTallies.Set(ctx, types.NewProofStoreKey(proof.height, proof.index), batch.tallies[proof]); err != nil {
			return err
		}
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	validatorCodec := k.stakingKeeper.ValidatorAddressCodec()
	var revelationCount, equivocationCount float32
	for _, event := range batch.events {
		validator, err := validatorCodec.BytesToString(event.validator)
		if err != nil {
			return err
		}
		if event.equivocation {
			equivocationCount++
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				types.EventTypeValidatorEquivocated,
				sdk.NewAttribute(types.AttributeKeyValidator, validator),
				sdk.NewAttribute(types.AttributeKeyProofHeight, strconv.FormatUint(event.proof.height, 10)),
				sdk.NewAttribute(types.AttributeKeyProofIndex, strconv.FormatUint(uint64(event.proof.index), 10)),
			))
			continue
		}
		revelationCount++
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeCommitmentRevealed,
			sdk.NewAttribute(types.AttributeKeyValidator, validator),
			sdk.NewAttribute(types.AttributeKeyCommitmentHeight, strconv.FormatUint(event.commitmentHeight, 10)),
		))
	}
	if revelationCount > 0 {
		telemetry.IncrCounter(revelationCount, types.ModuleName, "revelation", "accepted")
	}
	if equivocationCount > 0 {
		telemetry.IncrCounter(equivocationCount, types.ModuleName, "revelation", "equivocation")
	}

	return nil
}
