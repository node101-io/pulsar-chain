package abci

import (
	"errors"

	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	keyregistryTypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
	verificationTypes "github.com/node101-io/pulsar-chain/x/verification/types"
	votepersistenceTypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

// ProcessProposalHandler independently reconstructs and validates the reserved
// payload before accepting the proposal. A proposer may choose which optional
// verification entries fit, but it cannot decide whether they are authentic or
// state-valid; every validator repeats those checks before voting for the block.
func (h *ABCIHandler) ProcessProposalHandler() sdk.ProcessProposalHandler {

	return func(ctx sdk.Context, req *cometabci.RequestProcessProposal) (*cometabci.ResponseProcessProposal, error) {

		shouldValidateVoteExtensions, err := shouldRequireProposalPayloadAtHeight(ctx, req.GetHeight())
		if err != nil {
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, err
		}

		if !shouldValidateVoteExtensions {
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_ACCEPT}, nil
		}

		proposalHeight := req.GetHeight()
		// Vote extensions included in a proposal at height N are produced and
		// requested by consensus at height N-1. The payload must identify that
		// vote-extension height so validators reconstruct the same signed body.
		voteExtensionHeight := proposalHeight - 1

		body, err := h.constructVoteExtBody(ctx, voteExtensionHeight)
		if err != nil {
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, err
		}

		pl, payloadFound, err := extractPayload(req.Txs)
		if err != nil {
			if isInvalidProcessProposalError(err) {
				return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, nil
			}
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, err
		}
		if !payloadFound {
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, nil
		}
		if err := validatePayloadHeight(pl, voteExtensionHeight); err != nil {
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, nil
		}

		verifiedVotes, err := h.validatePayloadVoteExtensions(ctx, voteExtensionHeight, pl, body)
		if err != nil {
			if isInvalidProcessProposalError(err) {
				return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, nil
			}
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, err
		}

		if !hasAtLeastTwoThirdsPower(verifiedVotes.signedPower, verifiedVotes.totalPower) {
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, nil
		}
		// A block may contain no verification entries because sidecar work is
		// asynchronous and non-consensus. Once an entry is included, however, it
		// changes consensus state and must be authenticated, lifecycle-eligible,
		// and valid against the same pre-block state on every validator.
		if _, err := h.validateVerificationEntries(ctx, proposalHeight, pl, req.ProposedLastCommit); err != nil {
			if isInvalidProcessProposalError(err) {
				return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, nil
			}
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, err
		}

		return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_ACCEPT}, nil
	}

}

// Invalid proposal errors are deterministic rejection outcomes caused by bytes
// under proposer control. Internal application failures are returned as errors
// instead, so operators can distinguish a bad proposal from local corruption or
// unavailable state that requires attention.
func isInvalidProcessProposalError(err error) bool {
	return errors.Is(err, ErrInvalidPayload) ||
		errors.Is(err, ErrInvalidPayloadHeight) ||
		errors.Is(err, ErrVoteExtPayloadNotFound) ||
		errors.Is(err, ErrNotEnoughStakePower) ||
		errors.Is(err, keyregistryTypes.ErrValidatorNotRegistered) ||
		errors.Is(err, votepersistenceTypes.ErrInvalidVoteExtension) ||
		errors.Is(err, ErrInvalidCompositeVoteExtension) ||
		errors.Is(err, ErrInvalidVerificationPayload) ||
		errors.Is(err, ErrInvalidVerificationSignature) ||
		isVerificationActionError(err)
}

// isVerificationActionError classifies invalid proposer-controlled commitment
// and revelation data as proposal rejection rather than an internal application
// failure. Keeping this list explicit avoids accidentally masking storage or
// keeper errors as ordinary Byzantine input.
func isVerificationActionError(err error) bool {
	errorsToReject := []error{
		verificationTypes.ErrInvalidValidator,
		verificationTypes.ErrProofNotFound,
		verificationTypes.ErrProofHeightNotFound,
		verificationTypes.ErrInvalidCommitmentLength,
		verificationTypes.ErrInvalidCommitmentHeight,
		verificationTypes.ErrCommitmentAlreadyExists,
		verificationTypes.ErrCommitmentNotFound,
		verificationTypes.ErrCommitmentMismatch,
		verificationTypes.ErrInvalidRevelationCount,
		verificationTypes.ErrDuplicateCommitmentRevelation,
		verificationTypes.ErrInvalidLeafRevealMode,
		verificationTypes.ErrInvalidLeafHashLength,
		verificationTypes.ErrInvalidSaltLength,
		verificationTypes.ErrEarlyReveal,
		verificationTypes.ErrUselessRevelation,
		verificationTypes.ErrTooManyVotes,
		verificationTypes.ErrInvalidVoteIndex,
		verificationTypes.ErrDuplicateVoteIndex,
		verificationTypes.ErrNonCanonicalVoteOrdering,
	}
	for _, target := range errorsToReject {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
