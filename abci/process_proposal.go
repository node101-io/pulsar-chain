package vote_ext

import (
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (h *ABCIHandler) ProcessProposalHandler() sdk.ProcessProposalHandler {

	return func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {

		shouldValidateVoteExtensions, err := shouldRequireProposalPayloadAtHeight(ctx, req.GetHeight())
		if err != nil {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, err
		}

		if !shouldValidateVoteExtensions {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
		}

		body, err := h.constructVoteExtBody(ctx, req.GetHeight()-1)
		if err != nil {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, err
		}

		pl, err := extractPayload(req.Txs)
		if err != nil {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, err
		}

		isEnoughStakePower, err := h.checkStakePower(ctx, req.GetHeight(), pl, body)
		if err != nil {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, err
		}

		if !isEnoughStakePower {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, ErrNotEnoughStakePower
		}

		// Vote extension successfully verified
		return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
	}

}
