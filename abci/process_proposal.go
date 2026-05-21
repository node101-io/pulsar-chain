package abci

import (
	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
		// requested by consensus at height N-1.
		voteExtensionHeight := proposalHeight - 1

		body, err := h.constructVoteExtBody(ctx, voteExtensionHeight)
		if err != nil {
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, err
		}

		pl, payloadFound, err := extractPayload(req.Txs)
		if err != nil {
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, err
		}
		if !payloadFound {
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, ErrVoteExtPayloadNotFound
		}
		if err := validatePayloadHeight(pl, voteExtensionHeight); err != nil {
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, err
		}

		verifiedVotes, err := h.validatePayloadVoteExtensions(ctx, proposalHeight, pl, body)
		if err != nil {
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, err
		}

		if !hasAtLeastTwoThirdsPower(verifiedVotes.signedPower, verifiedVotes.totalPower) {
			return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_REJECT}, ErrNotEnoughStakePower
		}

		// Vote extension successfully verified
		return &cometabci.ResponseProcessProposal{Status: cometabci.ResponseProcessProposal_ACCEPT}, nil
	}

}
