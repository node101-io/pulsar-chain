package abci

import (
	"errors"

	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	keyregistryTypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
	verificationTypes "github.com/node101-io/pulsar-chain/x/verification/types"
	votepersistenceTypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

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
		if _, err := h.validateVerificationEntries(ctx, proposalHeight, pl, req.ProposedLastCommit); err != nil {
			if isInvalidProcessProposalError(err) {
				return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, nil
			}
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, err
		}

		return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_ACCEPT}, nil
	}

}

// Invalid proposal errors are normal consensus rejection outcomes. Internal
// application failures should still be returned as errors so the node can surface them.
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
