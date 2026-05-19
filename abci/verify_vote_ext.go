package abci

import (
	"fmt"

	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func (h *ABCIHandler) VerifyVoteExtensionHandler() sdk.VerifyVoteExtensionHandler {
	return func(ctx sdk.Context, req *cometabci.RequestVerifyVoteExtension) (*cometabci.ResponseVerifyVoteExtension, error) {

		shouldVerifyVoteExtension, err := shouldExtendVoteAtHeight(ctx, req.GetHeight())
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}

		if !shouldVerifyVoteExtension {
			if len(req.VoteExtension) == 0 {
				return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_ACCEPT}, nil
			}

			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, fmt.Errorf("rejected")
		}

		logger := ctx.Logger()

		logger.Info("vote ext verify called")

		cosmosValidatorPubKey, err := h.getValidatorPublicKey(ctx, req.ValidatorAddress)
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}

		exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, cosmosValidatorPubKey)
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}

		if !exists {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, types.ErrValidatorNotRegistered
		}

		minaKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, cosmosValidatorPubKey)
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}

		body, err := h.constructVoteExtBody(ctx, req.GetHeight())
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}
		poseidon := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

		sigValidity := verifyVoteExtSig(poseidon, req.VoteExtension, body, minaKey, ActionsReducedRoot)
		if !sigValidity {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, types.ErrInvalidSignature
		}

		logger.Info("vote ext verified")

		return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_ACCEPT}, nil
	}
}
