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

		transitionSignature, err := h.secondaryKey.SignVoteExtBody(body)
		if err != nil {
			return nil, err
		}
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
