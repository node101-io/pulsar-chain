package vote_ext

import (
	"encoding/json"

	"cosmossdk.io/errors"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

// PrepareProposalHandler injects the collected vote-extensions (for height-1)
// as the very first transaction of the proposal block.
// A simple JSON payload prefixed by "VOTEEXT:" is used; this is *not* part of
// consensus state and will be verified by ProcessProposal on peers. The state
// root cache is seeded in ProcessProposal, not in PrepareProposal.
func (h *VoteExtHandler) PrepareProposalHandler() sdk.PrepareProposalHandler {

	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {

		// vote-extensions for previous height (H-1)
		targetHeight := uint64(req.GetHeight() - 1)
		votes := h.fetchVotes(targetHeight)
		if len(votes) == 0 {
			return &abci.ResponsePrepareProposal{Txs: req.Txs}, nil
		}

		pl := payload{Height: targetHeight, Votes: votes}
		bz, err := json.Marshal(pl)
		if err != nil {
			return nil, errors.Wrap(types.ErrFailedToMarshal, "")
		}

		// prefix makes it easier to identify the vote extension
		extTx := append(types.VoteExtMarker, bz...)

		// prepend to existing txs
		txs := make([][]byte, 0, len(req.Txs)+1)
		txs = append(txs, extTx)
		txs = append(txs, req.Txs...)

		return &abci.ResponsePrepareProposal{Txs: txs}, nil
	}
}
