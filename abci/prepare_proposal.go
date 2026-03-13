package vote_ext

import (
	"encoding/hex"
	"encoding/json"

	"cosmossdk.io/errors"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

// PrepareProposalHandler injects the collected vote-extensions (for height-1)
// as the very first transaction of the proposal block.
// A simple JSON payload prefixed by "VOTEEXT:" is used; this is *not* part of
// consensus state and will be verified by ProcessProposal on peers.
func (h *VoteExtHandler) PrepareProposalHandler() sdk.PrepareProposalHandler {
	type payload struct {
		Height uint64            `json:"height"`
		Votes  map[string][]byte `json:"votes"`
	}

	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		ctx.Logger().Info("PrepareProposalHandler:start", "height", req.GetHeight())
		ctx.Logger().Info("App hash in prepare proposal", "appHash", hex.EncodeToString(ctx.BlockHeader().AppHash))

		// vote-extensions for previous height (H-1)
		targetHeight := uint64(req.GetHeight() - 1)
		votes := h.fetchVotes(targetHeight)
		if len(votes) == 0 {
			ctx.Logger().Info("No votes for previous height, accepting proposal", "looking for", targetHeight, "proposal height", req.GetHeight())
			h.stateRoots[req.GetHeight()] = ctx.BlockHeader().AppHash
			ctx.Logger().Info("PrepareProposalHandler: State root has been set", "stateRoot", hex.EncodeToString(h.stateRoots[req.GetHeight()]), "height", req.GetHeight())

			return &abci.ResponsePrepareProposal{Txs: req.Txs}, nil
		}

		pl := payload{Height: targetHeight, Votes: votes}
		bz, err := json.Marshal(pl)
		if err != nil {
			ctx.Logger().Info("Failed to marshal payload", "error", err)
			return nil, errors.Wrap(types.ErrFailedToMarshal, "")
		}

		// prefix makes it easier to identify the vote extension
		extTx := append(types.VoteExtMarker, bz...)

		// prepend to existing txs
		txs := make([][]byte, 0, len(req.Txs)+1)
		txs = append(txs, extTx)
		txs = append(txs, req.Txs...)

		h.stateRoots[req.GetHeight()] = ctx.BlockHeader().AppHash
		ctx.Logger().Info("PrepareProposalHandler: State root has been set", "stateRoot", hex.EncodeToString(h.stateRoots[req.GetHeight()]), "height", req.GetHeight())

		return &abci.ResponsePrepareProposal{Txs: txs}, nil
	}
}
