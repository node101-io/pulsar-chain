package abci

import (
	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (h *ABCIHandler) PrepareProposalHandler() sdk.PrepareProposalHandler {

	return func(ctx sdk.Context, req *cometabci.RequestPrepareProposal) (*cometabci.ResponsePrepareProposal, error) {

		shouldIncludeVoteExtensions, err := shouldRequireProposalPayloadAtHeight(ctx, req.GetHeight())
		if err != nil {
			return nil, err
		}

		if !shouldIncludeVoteExtensions {
			return &cometabci.ResponsePrepareProposal{Txs: req.Txs}, nil
		}

		votes := req.LocalLastCommit.Votes
		pl, err := h.constructPayload(ctx, req.GetHeight(), votes)
		if err != nil {
			return &cometabci.ResponsePrepareProposal{Txs: req.Txs}, err
		}

		bz, err := pl.Marshal()
		if err != nil {
			return nil, err
		}

		// prefix makes it easier to identify the vote extension
		extTx := append([]byte(VoteExtMarker), bz...)

		// prepend to existing txs
		txs := make([][]byte, 0, len(req.Txs)+1)
		txs = append(txs, extTx)
		txs = append(txs, req.Txs...)

		return &cometabci.ResponsePrepareProposal{Txs: txs}, nil
	}
}
