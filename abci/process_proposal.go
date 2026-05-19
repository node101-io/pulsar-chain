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

		voteExtensionHeight := req.GetHeight() - 1

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

		verifiedVotes, err := h.validatePayloadVoteExtensions(ctx, req.GetHeight(), pl, body)
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
