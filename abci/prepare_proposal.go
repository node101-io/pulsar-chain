package vote_ext

import (
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (h *AbciHandler) PrepareProposalHandler() sdk.PrepareProposalHandler {

	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {

		voteExtsForGivenBlock := make(map[string][]byte)
		targetHeight := uint64(req.GetHeight() - 1)

		valInfo, err := h.getValidatorSet(ctx, req.Height)
		if err != nil {
			return &abci.ResponsePrepareProposal{Txs: req.Txs}, err
		}

		for i, val := range valInfo {

			exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, val.ConsensusAddr)
			if err != nil {
				return &abci.ResponsePrepareProposal{Txs: req.Txs}, err
			}
			if !exists {
				return &abci.ResponsePrepareProposal{Txs: req.Txs}, err
			}

			votes := req.LocalLastCommit.Votes

			voteExtsForGivenBlock[string(votes[i].Validator.Address)] = votes[i].ExtensionSignature
		}

		if len(voteExtsForGivenBlock) == 0 {
			return &abci.ResponsePrepareProposal{Txs: req.Txs}, err
		}

		pl := payload{Height: targetHeight, Votes: voteExtsForGivenBlock}
		bz, err := json.Marshal(pl)
		if err != nil {
			return nil, fmt.Errorf("")
		}

		// prefix makes it easier to identify the vote extension
		extTx := append(VoteExtMarker, bz...)

		// prepend to existing txs
		txs := make([][]byte, 0, len(req.Txs)+1)
		txs = append(txs, extTx)
		txs = append(txs, req.Txs...)

		return &abci.ResponsePrepareProposal{Txs: txs}, nil
	}
}
