package abci

import (
	"errors"

	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	verificationTypes "github.com/node101-io/pulsar-chain/x/verification/types"
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

		var reveal *verificationTypes.ProofCommitmentReveal

		storedReveal, err := h.revealStore.Get(req.GetHeight() - 1)
		if err != nil {
			if !errors.Is(err, ErrRevealNotFound) {
				return nil, err
			}
		} else {
			reveal = &storedReveal
		}

		signature, err := h.secondaryKey.SignVoteExtension(body, proofCommitment)
		if err != nil {
			return nil, err
		}

		bz, err := (&VoteExtension{
			Signature:       signature,
			ProofCommitment: proofCommitment,
			Reveal:          reveal,
		}).Marshal()
		if err != nil {
			return nil, err
		}

		return &cometabci.ResponseExtendVote{VoteExtension: bz}, nil
	}
}
