package abci

import (
	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (h *ABCIHandler) PreBlocker() sdk.PreBlocker {
	return func(ctx sdk.Context, req *cometabci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {

		shouldPersistVoteExtensions, err := shouldRequireProposalPayloadAtHeight(ctx, req.GetHeight())
		if err != nil {
			return nil, err
		}
		if !shouldPersistVoteExtensions {
			return &sdk.ResponsePreBlock{}, nil
		}

		pl, payloadFound, err := extractPayload(req.Txs)
		if err != nil {
			return nil, err
		}
		if !payloadFound {
			return nil, ErrVoteExtPayloadNotFound
		}

		body, err := h.constructVoteExtBody(ctx, req.GetHeight()-1)
		if err != nil {
			return nil, err
		}

		verifiedVotes, err := h.validatePayloadVotes(ctx, req.GetHeight(), pl, body)
		if err != nil {
			return nil, err
		}

		if !hasAtLeastTwoThirdsPower(verifiedVotes.signedPower, verifiedVotes.totalPower) {
			return nil, ErrNotEnoughStakePower
		}

		if err := h.votePersistenceKeeper.Clear(ctx); err != nil {
			return nil, err
		}

		for _, vote := range verifiedVotes.votes {
			if err := h.votePersistenceKeeper.SetVote(ctx, req.GetHeight()-2, vote.minaPublicKey, vote.voteExtension); err != nil {
				return nil, err
			}
		}

		return &sdk.ResponsePreBlock{}, nil
	}
}
