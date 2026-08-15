package abci

import (
	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (h *ABCIHandler) ExtendVoteHandler() sdk.ExtendVoteHandler {
	return func(ctx sdk.Context, req *cometabci.RequestExtendVote) (*cometabci.ResponseExtendVote, error) {

		shouldExtendVote, err := shouldExtendVoteAtHeight(ctx, req.GetHeight())
		if err != nil {
			return nil, err
		}

		if !shouldExtendVote {
			return &cometabci.ResponseExtendVote{VoteExtension: []byte{}}, nil
		}

		body, err := h.constructVoteExtBody(ctx, req.GetHeight())
		if err != nil {
			return nil, err
		}

		proofCommitment, err := h.GenerateCommitmentForVerifiedProofs(ctx)
		if err != nil {
			return nil, err
		}

		signature, err := h.secondaryKey.SignVoteExtension(body, proofCommitment)
		if err != nil {
			return nil, err
		}

		bz, err := (&VoteExtension{
			Signature:       signature,
			ProofCommitment: proofCommitment,
		}).Marshal()
		if err != nil {
			return nil, err
		}

		return &cometabci.ResponseExtendVote{VoteExtension: bz}, nil
	}
}
