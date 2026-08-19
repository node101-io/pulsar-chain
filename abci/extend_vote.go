package abci

import (
	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ExtendVoteHandler signs the mandatory Mina state transition, adds any optional
// verification actions for the next block, and returns one composite envelope.
// CometBFT signs that complete envelope after this handler returns, binding both
// components to the validator's consensus key, height, round, and chain ID.
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

		transitionSignature, err := h.secondaryKey.SignVoteExtBody(body)
		if err != nil {
			return nil, err
		}
		// The Mina signature remains mandatory because the bridge already depends
		// on it for consensus. Verification is deliberately nullable: sidecar
		// timeouts, unfinished proofs, or local-journal failures only cost this
		// validator a verification opportunity and must not stop block production.
		extension := &CompositeVoteExtension{
			ProtocolVersion:     CompositeVoteExtensionVersion,
			TransitionSignature: transitionSignature,
			VerificationPayload: h.buildVerificationPayload(ctx, req.GetHeight()),
		}
		bz, err := encodeCompositeVoteExtension(extension)
		if err != nil {
			return nil, err
		}

		return &cometabci.ResponseExtendVote{VoteExtension: bz}, nil
	}
}
