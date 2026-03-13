package vote_ext

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

		// If height is 1, we won't have any votes thus skip the proposal
		if req.GetHeight() == 1 {
			ctx.Logger().Info("Height is 1, skipping proposal", "height", req.GetHeight())
			h.stateRoots[0] = make([]byte, 32)
			genesisStateRoot, err := hex.DecodeString(GenesisStateRoot)
			if err != nil {
				ctx.Logger().Info("Failed to decode genesis state root", "error", err)
				return nil, fmt.Errorf("failed to decode genesis state root: %w", err)
			}
			// Set the state root to the genesis state root
			h.stateRoots[req.GetHeight()] = genesisStateRoot
			ctx.Logger().Info("PrepareProposalHandler: State root has been set", "stateRoot", hex.EncodeToString(h.stateRoots[0]), "height", 0)

			return &abci.ResponsePrepareProposal{Txs: req.Txs}, nil
		}

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
			return nil, fmt.Errorf("marshal payload: %w", err)
		}

		// prefix makes it easier to identify the vote extension
		marker := []byte("VOTEEXT:")
		extTx := append(marker, bz...)

		// prepend to existing txs
		txs := make([][]byte, 0, len(req.Txs)+1)
		txs = append(txs, extTx)
		txs = append(txs, req.Txs...)

		h.stateRoots[req.GetHeight()] = ctx.BlockHeader().AppHash
		ctx.Logger().Info("PrepareProposalHandler: State root has been set", "stateRoot", hex.EncodeToString(h.stateRoots[req.GetHeight()]), "height", req.GetHeight())

		return &abci.ResponsePrepareProposal{Txs: txs}, nil
	}
}
