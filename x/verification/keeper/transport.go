package keeper

import (
	"context"
	"encoding/hex"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

// verificationPayloadPlan is the complete, prevalidated state transition for
// one authenticated validator payload. Keeping a plan separate from applying
// it lets proposal validation simulate several validator actions in order
// without exposing partial consensus writes.
type verificationPayloadPlan struct {
	validator          []byte
	targetHeight       uint64
	commitment         []byte
	revelationBatch    ValidatedRevelationBatch
	hasRevelationBatch bool
}

// ValidateVerificationPayload checks a commitment/revelation payload without
// mutating consensus state. Proposal construction uses this as an early filter
// so invalid optional verification data can be omitted while the validator's
// mandatory Mina transition signature is still included.
func (k Keeper) ValidateVerificationPayload(
	ctx context.Context,
	validator []byte,
	targetHeight uint64,
	commitment []byte,
	revelations []types.CommitmentRevelation,
) error {
	_, err := k.planVerificationPayload(ctx, validator, targetHeight, commitment, revelations)
	return err
}

// ApplyVerificationPayload validates and applies the complete payload
// atomically. A failure in either the commitment or any revelation leaves all
// verification state unchanged. This is required because a payload may both
// reveal old roots and submit the next root; accepting only one half would make
// validators observe different protocol histories.
func (k Keeper) ApplyVerificationPayload(
	ctx context.Context,
	validator []byte,
	targetHeight uint64,
	commitment []byte,
	revelations []types.CommitmentRevelation,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height, err := blockHeight(sdkCtx)
	if err != nil {
		return err
	}
	if height != targetHeight {
		return types.ErrInvalidCommitmentHeight
	}
	cacheCtx, write := sdkCtx.CacheContext()
	plan, err := k.planVerificationPayload(cacheCtx, validator, targetHeight, commitment, revelations)
	if err != nil {
		return err
	}
	if len(plan.commitment) != 0 {
		if err := k.submitCommitment(cacheCtx, plan.validator, plan.targetHeight, plan.commitment); err != nil {
			return err
		}
		validatorString, err := k.stakingKeeper.ValidatorAddressCodec().BytesToString(plan.validator)
		if err != nil {
			return err
		}
		cacheCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventTypeCommitmentSubmitted,
			sdk.NewAttribute(types.AttributeKeyValidator, validatorString),
			sdk.NewAttribute(types.AttributeKeyCommitmentHeight, strconv.FormatUint(plan.targetHeight, 10)),
			sdk.NewAttribute(types.AttributeKeyCommitment, hex.EncodeToString(plan.commitment)),
		))
	}
	if plan.hasRevelationBatch {
		if err := k.ApplyRevelationPlans(cacheCtx, plan.revelationBatch); err != nil {
			return err
		}
	}
	write()

	return nil
}

func (k Keeper) planVerificationPayload(
	ctx context.Context,
	validator []byte,
	targetHeight uint64,
	commitment []byte,
	revelations []types.CommitmentRevelation,
) (verificationPayloadPlan, error) {
	if len(commitment) == 0 && len(revelations) == 0 {
		return verificationPayloadPlan{}, types.ErrInvalidRevelationCount
	}
	plan := verificationPayloadPlan{
		validator:    append([]byte(nil), validator...),
		targetHeight: targetHeight,
		commitment:   append([]byte(nil), commitment...),
	}
	if len(commitment) != 0 {
		if err := k.validateSubmitCommitment(ctx, validator, targetHeight, commitment); err != nil {
			return verificationPayloadPlan{}, err
		}
	}
	if len(revelations) == 0 {
		return plan, nil
	}
	if len(revelations) > types.MaxRevelationsPerBatch {
		return verificationPayloadPlan{}, types.ErrInvalidRevelationCount
	}

	// One payload cannot reveal the same commitment twice, even if its leaf
	// encodings differ. The batch must describe one unambiguous opening for each
	// commitment height.
	seenHeights := make(map[uint64]struct{}, len(revelations))
	revelationPlans := make([]RevelationPlan, 0, len(revelations))
	for _, revelation := range revelations {
		if _, exists := seenHeights[revelation.CommitmentHeight]; exists {
			return verificationPayloadPlan{}, types.ErrDuplicateCommitmentRevelation
		}
		seenHeights[revelation.CommitmentHeight] = struct{}{}
		revelationPlan, err := k.ValidateCommitmentRevelation(ctx, validator, targetHeight, revelation)
		if err != nil {
			return verificationPayloadPlan{}, err
		}
		revelationPlans = append(revelationPlans, revelationPlan)
	}
	batch, err := k.ValidateRevelationPlans(ctx, revelationPlans)
	if err != nil {
		return verificationPayloadPlan{}, err
	}
	plan.revelationBatch = batch
	plan.hasRevelationBatch = true

	return plan, nil
}
