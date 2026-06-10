package abci

import (
	"errors"

	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/poseidon"
	keyregistryTypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
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

			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, nil
		}

		cosmosValidatorPubKey, err := h.getConsPubKeyByConsAddr(ctx, req.ValidatorAddress)
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}

		exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, cosmosValidatorPubKey)
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}

		if !exists {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, nil
		}

		minaKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, cosmosValidatorPubKey)
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}

		body, err := h.constructVoteExtBody(ctx, req.GetHeight())
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}
		poseidonHash := poseidon.NewPoseidon()

		if err := verifyVoteExtSig(poseidonHash, req.VoteExtension, body, minaKey, ActionsReducedRoot, h.networkID); err != nil {
			if errors.Is(err, keyregistryTypes.ErrValidatorNotRegistered) ||
				errors.Is(err, ErrInvalidVoteExtSignatureEncoding) ||
				errors.Is(err, ErrInvalidVoteExtSignature) {
				return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, nil
			}

			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}

		return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_ACCEPT}, nil
	}
}
