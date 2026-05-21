package abci

import (
	"fmt"

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
			return nil, err
		}

		bz, err := pl.Marshal()
		if err != nil {
			return nil, err
		}

		// The marker reserves the first transaction slot for the internal
		// vote-extension payload; remaining entries are normal user transactions.
		extTx := append(voteExtMarkerBytes[:len(voteExtMarkerBytes):len(voteExtMarkerBytes)], bz...)
		if int64(len(extTx)) > req.MaxTxBytes {
			return nil, fmt.Errorf("%w: payload tx size %d exceeds max tx bytes %d", ErrVoteExtPayloadTooLarge, len(extTx), req.MaxTxBytes)
		}

		txs := make([][]byte, 0, len(req.Txs)+1)
		txs = append(txs, extTx)

		totalBytes := int64(len(extTx))
		for _, tx := range req.Txs {
			txSize := int64(len(tx))
			if totalBytes+txSize > req.MaxTxBytes {
				break
			}
			txs = append(txs, tx)
			totalBytes += txSize
		}

		return &cometabci.ResponsePrepareProposal{Txs: txs}, nil
	}
}
