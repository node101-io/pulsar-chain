package vote_ext

import (
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (h *AbciHandler) PrepareProposalHandler() sdk.PrepareProposalHandler {

	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {

		// to construct the payload we need to get the N-2th block's vote extensions.
		// Hence, enabling prepare proposal on blocks < 3 will result in error.
		if req.Height < 3 {
			return &abci.ResponsePrepareProposal{Txs: req.Txs}, nil
		}

		votes := req.LocalLastCommit.Votes
		pl, err := h.constructPayload(ctx, req.GetHeight(), votes)
		if err != nil {
			return &abci.ResponsePrepareProposal{Txs: req.Txs}, err
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

		return &abci.ResponsePrepareProposal{Txs: txs}, nil
	}
}
