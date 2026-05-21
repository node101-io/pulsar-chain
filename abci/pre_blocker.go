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

		proposalHeight := req.GetHeight()
		// Vote extensions included in a proposal at height N are produced and
		// requested by consensus at height N-1.
		voteExtensionHeight := proposalHeight - 1
		// Each vote extension body signs the state transition produced two
		// blocks earlier, so persistence is keyed by the proof target height N-2.
		proofTargetHeight := proposalHeight - 2

		if err := validatePayloadHeight(pl, voteExtensionHeight); err != nil {
			return nil, err
		}

		body, err := h.constructVoteExtBody(ctx, voteExtensionHeight)
		if err != nil {
			return nil, err
		}

		verifiedVotes, err := h.validatePayloadVoteExtensions(ctx, req.GetHeight(), pl, body)
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
			if err := h.votePersistenceKeeper.SetVote(ctx, proofTargetHeight, vote.minaPublicKey, vote.voteExtension); err != nil {
				return nil, err
			}
		}

		return &sdk.ResponsePreBlock{}, nil
	}
}
