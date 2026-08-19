package abci

import (
	"bytes"
	"fmt"

	cometabci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	protoio "github.com/cosmos/gogoproto/io"

	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

// validatedVerificationAction contains the operator identity derived from the
// historical validator set and its authenticated verification payload. Keeping
// the derived operator beside the payload prevents FinalizeBlock from trusting
// an address supplied directly by the proposer.
type validatedVerificationAction struct {
	operator []byte
	payload  *verificationtypes.VerificationVoteExtensionPayload
}

// validateVerificationEntries authenticates every included action through four
// independent bindings: the validator participated in the last commit, existed
// in the historical validator set, signed the complete envelope with its
// CometBFT key, and supplied the same Mina signature recorded in the mandatory
// payload. These checks prevent a proposer from inventing, copying, or mixing
// verification actions. The function also simulates actions sequentially in a
// cache so conflicts between otherwise valid entries are found without mutating
// caller state.
func (h *ABCIHandler) validateVerificationEntries(
	ctx sdk.Context,
	targetHeight int64,
	payload Payload,
	lastCommit cometabci.CommitInfo,
) ([]validatedVerificationAction, error) {
	if len(payload.VerificationEntries) == 0 {
		return nil, nil
	}
	if h.verificationKeeper == nil || targetHeight < 1 || lastCommit.Round < 0 {
		return nil, ErrInvalidVerificationPayload
	}
	validatorSet, err := h.getValidatorSet(ctx, targetHeight-1)
	if err != nil {
		return nil, err
	}
	validatorsByAddress, err := validatorSetByConsensusAddress(validatorSet)
	if err != nil {
		return nil, err
	}

	// Only validators whose votes actually committed the previous block may
	// contribute verification actions to this proposal. Being present in the
	// validator set is not enough: absent and nil votes did not authorize payload
	// bytes for this round.
	committed := make(map[string]struct{}, len(lastCommit.Votes))
	seenValidators := make(map[string]struct{}, len(lastCommit.Votes))
	for _, vote := range lastCommit.Votes {
		if len(vote.Validator.Address) == 0 || vote.Validator.Power <= 0 {
			return nil, ErrInvalidVerificationPayload
		}
		key := string(vote.Validator.Address)
		if _, duplicate := seenValidators[key]; duplicate {
			return nil, ErrInvalidVerificationPayload
		}
		seenValidators[key] = struct{}{}
		if vote.BlockIdFlag == cmtproto.BlockIDFlagCommit {
			committed[key] = struct{}{}
		}
	}

	// Bind each optional composite entry to the exact Mina signature already
	// accepted in the mandatory payload list. This prevents a proposer from
	// combining one validator's optional envelope with another mandatory vote or
	// injecting an optional-only entry that bypasses the main payload checks.
	mandatory := make(map[string][]byte, len(payload.VoteExtensions))
	for _, vote := range payload.VoteExtensions {
		if vote == nil || len(vote.ConsensusPublicKey) == 0 || len(vote.VoteExtension) == 0 {
			return nil, ErrInvalidPayload
		}
		key := string(vote.ConsensusPublicKey)
		if _, duplicate := mandatory[key]; duplicate {
			return nil, ErrInvalidPayload
		}
		mandatory[key] = vote.VoteExtension
	}

	// Apply into a throwaway cache while validating so later entries observe
	// earlier staged state. This catches cross-validator conflicts such as a
	// duplicate root or an invalid batched transition, while no writes escape
	// ProcessProposal's read-only decision.
	cacheCtx, _ := ctx.CacheContext()
	actions := make([]validatedVerificationAction, 0, len(payload.VerificationEntries))
	var previousAddress []byte
	for index, entry := range payload.VerificationEntries {
		if entry == nil || len(entry.ValidatorAddress) == 0 ||
			entry.SourceHeight != targetHeight-1 || entry.Round != lastCommit.Round {
			return nil, ErrInvalidVerificationPayload
		}
		if index > 0 && bytes.Compare(previousAddress, entry.ValidatorAddress) >= 0 {
			return nil, ErrInvalidVerificationPayload
		}
		previousAddress = entry.ValidatorAddress
		if _, ok := committed[string(entry.ValidatorAddress)]; !ok {
			return nil, ErrInvalidVerificationPayload
		}

		validator, ok := validatorsByAddress[string(entry.ValidatorAddress)]
		if !ok {
			return nil, ErrInvalidVerificationPayload
		}
		consensusPublicKey, err := validator.ConsPubKey()
		if err != nil {
			return nil, err
		}
		composite, err := decodeCompositeVoteExtension(entry.CompositeVoteExtension)
		if err != nil || composite.VerificationPayload == nil {
			return nil, ErrInvalidVerificationPayload
		}
		transitionSignature, ok := mandatory[string(consensusPublicKey.Bytes())]
		if !ok || !bytes.Equal(transitionSignature, composite.TransitionSignature) {
			return nil, ErrInvalidVerificationPayload
		}
		if err := verifyCometVoteExtensionSignature(
			ctx.ChainID(), consensusPublicKey, entry.SourceHeight, entry.Round,
			entry.CompositeVoteExtension, entry.ExtensionSignature,
		); err != nil {
			return nil, err
		}
		if err := validateVerificationPayloadStructure(composite.VerificationPayload, uint64(targetHeight)); err != nil {
			return nil, err
		}
		operator, err := sdk.ValAddressFromBech32(validator.GetOperator())
		if err != nil {
			return nil, err
		}
		if err := h.verificationKeeper.ApplyVerificationPayload(
			cacheCtx,
			operator,
			uint64(targetHeight),
			composite.VerificationPayload.Commitment,
			composite.VerificationPayload.Revelations,
		); err != nil {
			return nil, err
		}
		actions = append(actions, validatedVerificationAction{
			operator: append([]byte(nil), operator...),
			payload:  composite.VerificationPayload,
		})
	}

	return actions, nil
}

// verifyCometVoteExtensionSignature reconstructs CometBFT's canonical sign
// bytes and verifies the signature over the entire composite envelope. Height,
// round, and chain ID are part of those bytes, preventing replay on another
// round, block, or chain; the optional verification payload cannot be edited
// without invalidating the signature.
func verifyCometVoteExtensionSignature(
	chainID string,
	publicKey interface{ VerifySignature([]byte, []byte) bool },
	height int64,
	round int32,
	extension,
	signature []byte,
) error {
	if len(chainID) == 0 || len(signature) == 0 {
		return ErrInvalidVerificationSignature
	}
	canonical := cmtproto.CanonicalVoteExtension{
		Extension: extension,
		Height:    height,
		Round:     int64(round),
		ChainId:   chainID,
	}
	var signBytes bytes.Buffer
	if err := protoio.NewDelimitedWriter(&signBytes).WriteMsg(&canonical); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationSignature, err)
	}
	if !publicKey.VerifySignature(signBytes.Bytes(), signature) {
		return ErrInvalidVerificationSignature
	}

	return nil
}

// applyVerificationActions applies actions already authenticated and
// prevalidated during proposal processing. The keeper validates them again on
// the FinalizeBlock cache because ProcessProposal cannot authorize state writes
// and state may never be trusted solely from the proposer-facing path.
func (h *ABCIHandler) applyVerificationActions(
	ctx sdk.Context,
	actions []validatedVerificationAction,
) error {
	for _, action := range actions {
		if err := h.verificationKeeper.ApplyVerificationPayload(
			ctx,
			action.operator,
			action.payload.TargetHeight,
			action.payload.Commitment,
			action.payload.Revelations,
		); err != nil {
			return err
		}
	}
	return nil
}
