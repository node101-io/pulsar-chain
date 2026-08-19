package abci

import (
	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// PreBlocker persists mandatory Mina votes and applies authenticated verification
// actions before normal transaction execution. Both write sets share one cache
// so a failure cannot leave the bridge-facing vote state and verification state
// describing different versions of the same committed block.
func (h *ABCIHandler) PreBlocker() sdk.PreBlocker {
	return func(ctx sdk.Context, req *cometabci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {

		shouldPersistVoteExtensions, err := shouldRequireProposalPayloadAtHeight(ctx, req.GetHeight())
		if err != nil {
			return nil, err
		}
		if !shouldPersistVoteExtensions {
			// A snapshot is taken before proof transactions execute so every proof
			// accepted in this block uses the validator set visible at the block
			// boundary. EndBlock removes it when no proof was accepted, avoiding
			// permanent state for heights that never enter verification.
			if h.verificationKeeper != nil {
				if err := h.verificationKeeper.CreateValidatorSnapshot(ctx, uint64(req.GetHeight())); err != nil {
					return nil, err
				}
			}
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
		// requested by consensus at height N-1. That is the height encoded in
		// Payload.vote_extension_height and used to reconstruct the signed body.
		voteExtensionHeight := proposalHeight - 1
		// A vote extension at height N-1 signs the transition from state N-3
		// to state N-2, so persistence is keyed by the signed source state height.
		signedStateHeight := voteExtensionHeight - 2

		if err := validatePayloadHeight(pl, voteExtensionHeight); err != nil {
			return nil, err
		}

		body, err := h.constructVoteExtBody(ctx, voteExtensionHeight)
		if err != nil {
			return nil, err
		}

		verifiedVotes, err := h.validatePayloadVoteExtensions(ctx, voteExtensionHeight, pl, body)
		if err != nil {
			return nil, err
		}

		if !hasAtLeastTwoThirdsPower(verifiedVotes.signedPower, verifiedVotes.totalPower) {
			return nil, ErrNotEnoughStakePower
		}
		if h.verificationKeeper == nil {
			if err := h.votePersistenceKeeper.Clear(ctx); err != nil {
				return nil, err
			}
			for _, vote := range verifiedVotes.votes {
				if err := h.votePersistenceKeeper.SetVote(ctx, signedStateHeight, vote.minaPublicKey, vote.voteExtension); err != nil {
					return nil, err
				}
			}
			return &sdk.ResponsePreBlock{}, nil
		}

		// Mina vote persistence, verification actions, and the next proof-height
		// snapshot are committed atomically. The actions are validated once more in
		// this cache, then written only if every component succeeds.
		cacheCtx, write := ctx.CacheContext()
		actions, err := h.validateVerificationEntries(cacheCtx, proposalHeight, pl, req.DecidedLastCommit)
		if err != nil {
			return nil, err
		}

		if err := h.votePersistenceKeeper.Clear(cacheCtx); err != nil {
			return nil, err
		}

		for _, vote := range verifiedVotes.votes {
			if err := h.votePersistenceKeeper.SetVote(cacheCtx, signedStateHeight, vote.minaPublicKey, vote.voteExtension); err != nil {
				return nil, err
			}
		}
		if err := h.applyVerificationActions(cacheCtx, actions); err != nil {
			return nil, err
		}
		if h.verificationKeeper != nil {
			if err := h.verificationKeeper.CreateValidatorSnapshot(cacheCtx, uint64(proposalHeight)); err != nil {
				return nil, err
			}
		}
		write()

		return &sdk.ResponsePreBlock{}, nil
	}
}
