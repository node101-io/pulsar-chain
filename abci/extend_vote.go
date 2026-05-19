package vote_ext

import (
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (h *ABCIHandler) ExtendVoteHandler() sdk.ExtendVoteHandler {
	return func(ctx sdk.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {

		shouldExtendVote, err := shouldExtendVoteAtHeight(ctx, req.GetHeight())
		if err != nil {
			return nil, err
		}

		if !shouldExtendVote {
			return &abci.ResponseExtendVote{VoteExtension: []byte{}}, nil
		}

		body, err := h.constructVoteExtBody(ctx, req.GetHeight())
		if err != nil {
			return nil, err
		}

		bz, err := h.secondaryKey.SignVoteExtBody(body)
		if err != nil {
			return nil, err
		}

		return &abci.ResponseExtendVote{VoteExtension: bz}, nil
	}
}
