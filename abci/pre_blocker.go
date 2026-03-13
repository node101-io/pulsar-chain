package abci

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

// PreBlocker persists the verified vote-extensions of height H-1 into the
// on-chain KVStore (VoteExt map) so that they can be queried externally.
// After persistence, in-memory cache for that height is cleared.
func (h *VoteExtHandler) PreBlocker() sdk.PreBlocker {
	return func(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
		ctx.Logger().Info("PreBlocker:start", "height", req.GetHeight())

		// If height is 1, we won't have any votes thus skip the proposal
		if req.GetHeight() == 1 {
			ctx.Logger().Info("Height is 1, skipping proposal by setting the state root", "height", req.GetHeight())
			h.stateRoots[req.GetHeight()] = ctx.BlockHeader().AppHash
			ctx.Logger().Info("PreBlocker: State root has been set", "stateRoot", hex.EncodeToString(h.stateRoots[req.GetHeight()]), "height", req.GetHeight())
			return &sdk.ResponsePreBlock{}, nil
		}

		targetHeight := uint64(req.GetHeight() - 1)
		votes := h.fetchVotes(targetHeight)
		if len(votes) == 0 {
			// log no votes
			ctx.Logger().Info("No votes", "height", targetHeight)
			return &sdk.ResponsePreBlock{}, nil
		}

		ctx.Logger().Info("PreBlocker", "persistHeight", targetHeight, "voteCount", len(votes))

		for consAddr, ext := range votes {
			var ve MinaSignatureVoteExt
			if err := json.Unmarshal(ext, &ve); err != nil {
				continue // skip malformed entry
			}

			sigHex := hex.EncodeToString(ve.Signature)
			idx := fmt.Sprintf("%d/%s", targetHeight, consAddr)

			record := types.VoteExt{
				Index:         idx,
				Height:        targetHeight,
				ValidatorAddr: ve.MinaAddress,
				Signature:     sigHex,
			}

			h.voteextKeeper.SetVoteExt(ctx, record)

			// Update height-based index mapping
			h.voteextKeeper.SetVoteExtIndex(ctx, targetHeight, idx)
		}

		// clear from memory after persisting
		h.deleteVotes(targetHeight)

		return &sdk.ResponsePreBlock{}, nil
	}
}
