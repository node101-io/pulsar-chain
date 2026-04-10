package vote_ext

import (
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
)

func (h *AbciHandler) VerifyVoteExtensionHandler() sdk.VerifyVoteExtensionHandler {
	return func(ctx sdk.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
		if req.GetHeight() < 3 {
			if len(req.VoteExtension) == 0 {
				return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_ACCEPT}, nil
			}

			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, fmt.Errorf("rejected")
		}

		cosmosValAddr := req.GetValidatorAddress()

		exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, cosmosValAddr)
		if err != nil {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, err
		}
		if !exists {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, err
		}

		poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

		minaKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, cosmosValAddr)
		if err != nil {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, err
		}

		valInfo, err := h.getValidatorSet(ctx, req.Height)
		if err != nil {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, err
		}

		setRoot, err := h.calculateValidatorSetRoot(ctx, valInfo, poseidonHash)
		if err != nil {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, err
		}

		sigValidity := MockSignatureVerify(VoteExtensionBody{
			NextValidatorSetHash: setRoot.Bytes(),
			CurrentStateRoot:     ctx.HeaderInfo().AppHash,
			NextBlockHeight:      req.Height,
		}, minaKey, ActionsReducedRoot)

		if !sigValidity {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, err
		}

		return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_ACCEPT}, nil
	}
}
