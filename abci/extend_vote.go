package vote_ext

import (
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
)

func (h *AbciHandler) ExtendVoteHandler() sdk.ExtendVoteHandler {
	return func(ctx sdk.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {

		cp := ctx.ConsensusParams()
		if cp.Abci == nil {
			return &abci.ResponseExtendVote{VoteExtension: []byte{}}, ErrUnableToReadConsensusParams
		}

		if req.Height < cp.Abci.VoteExtensionsEnableHeight+AdditionalVoteExtHeight {
			return &abci.ResponseExtendVote{VoteExtension: []byte{}}, nil
		}

		body, err := h.constructVoteExtBody(ctx, req.GetHeight())
		if err != nil {
			return nil, err
		}

		poseidon := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

		bz := h.secondaryKey.SignVoteExtBody(poseidon, body)

		return &abci.ResponseExtendVote{VoteExtension: bz}, nil
	}
}
