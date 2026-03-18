package vote_ext

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

		// If height is 1, we won't have any votes thus skip the proposal
		if req.GetHeight() == 1 {
			h.stateRoots[req.GetHeight()] = ctx.BlockHeader().AppHash
			return &sdk.ResponsePreBlock{}, nil
		}

		err := h.voteextKeeper.RemoveAllVoteExts(ctx)
		if err != nil {
			return nil, err
		}

		targetHeight := uint64(req.GetHeight() - 1)
		votes := h.fetchVotes(targetHeight)
		if len(votes) == 0 {
			return &sdk.ResponsePreBlock{}, nil
		}

		for minaAddress, ext := range votes {
			var ve MinaSignatureVoteExt
			if err := json.Unmarshal(ext, &ve); err != nil {
				continue
			}

			sigHex := hex.EncodeToString(ve.Signature)
			idx := fmt.Sprintf("%d/%s", targetHeight, minaAddress)

			record := types.VoteExt{
				Index:         idx,
				Height:        targetHeight,
				ValidatorAddr: ve.MinaAddress,
				Signature:     sigHex,
			}

			err := h.voteextKeeper.SetVoteExt(ctx, record)
			if err != nil {
				return nil, err
			}
		}

		// clear from memory after persisting
		h.deleteVotes(targetHeight)

		return &sdk.ResponsePreBlock{}, nil
	}
}
