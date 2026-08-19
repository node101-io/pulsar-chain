package abci

import (
	"errors"

	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/poseidon"
	keyregistryTypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// VerifyVoteExtensionHandler validates the composite envelope and mandatory
// Mina signature before CometBFT accepts the peer's vote extension. Verification
// data receives only context-free checks here because this callback is not where
// application state is committed; full eligibility, timing, and duplicate checks
// are repeated deterministically when the next proposal is processed.
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

		// The verification target is H+1 because ExtendVote(H) supplies actions
		// for the next proposal. Keeping this offset explicit avoids accepting a
		// valid commitment or revelation in the wrong lifecycle window.
		composite, err := decodeCompositeVoteExtension(req.VoteExtension)
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, nil
		}
		if err := validateVerificationPayloadStructure(composite.VerificationPayload, uint64(req.GetHeight()+1)); err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, nil
		}

		// Resolve the historical validator record so key rotation cannot make a
		// valid past-height signature depend on current staking state. Membership,
		// consensus key, and Mina key must all refer to the height that actually
		// produced the vote extension.
		validator, err := h.getValidatorByConsAddrAtHeight(ctx, req.GetHeight(), req.ValidatorAddress)
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}
		consensusPublicKey, err := validator.ConsPubKey()
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}
		cosmosValidatorPubKey := consensusPublicKey.Bytes()

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

		if err := verifyVoteExtSig(poseidonHash, composite.TransitionSignature, body, minaKey, h.networkID); err != nil {
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
