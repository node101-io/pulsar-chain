package vote_ext

import (
	"bytes"
	"encoding/json"

	"cosmossdk.io/errors"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

func (h *VoteExtHandler) VerifyVoteExtensionHandler() sdk.VerifyVoteExtensionHandler {
	return func(ctx sdk.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
		ctx.Logger().Info("VerifyVoteExtensionHandler:start", "height", req.GetHeight())
		// Unmarshal the extension payload
		var voteExt MinaSignatureVoteExt
		if err := json.Unmarshal(req.VoteExtension, &voteExt); err != nil {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, errors.Wrap(types.ErrMalformedVoteExtPayload, "")
		}

		// Log incoming vote extension for visibility
		ctx.Logger().Info("incoming vote extension", "voteExt", voteExt)

		err := h.verifyExtensionSig(ctx, req, voteExt)
		if err != nil {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, err
		}

		err = h.checkValidityOfVoteExtBody(ctx, req, voteExt)
		if err != nil {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, err
		}

		ctx.Logger().Info("vote extension verified", "validator", voteExt.MinaAddress)
		return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_ACCEPT}, nil
	}
}

func (h *VoteExtHandler) verifyExtensionSig(ctx sdk.Context, req *abci.RequestVerifyVoteExtension, voteExt MinaSignatureVoteExt) error {

	// Get the validator address from the request
	consAddr := sdk.ConsAddress(req.ValidatorAddress)

	// Log incoming vote-extension for visibility
	ctx.Logger().Info("VerifyVoteExtension", "height", req.GetHeight(), "validator", consAddr.String())

	exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, consAddr.Bytes())
	if err != nil {
		return errors.Wrap(types.ErrInternal, "")
	}

	// unknown validator – ignore the vote.
	if !exists {
		ctx.Logger().Info("unknown validator", consAddr.String())
		return errors.Wrap(types.ErrUnknownValidator, consAddr.String())
	}

	minaPublicKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, consAddr.Bytes())
	if err != nil {
		return errors.Wrap(types.ErrInternal, "")
	}
	// Initialize poseidon hash
	poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

	pubKey, err := new(keys.PublicKey).FromAddress(string(minaPublicKey))
	if err != nil {
		ctx.Logger().Info("failed to unmarshal mina public key for validator", "validator", consAddr.String(), "error", err)
		return errors.Wrap(types.ErrFailedToUnmarshal, consAddr.String())
	}

	// Check if the address is correct.
	pubKeyAddr, err := pubKey.ToAddress()
	if err != nil {
		ctx.Logger().Info("failed to convert public key to address", "validator", consAddr.String(), "error", err)
		return errors.Wrap(types.ErrFailedToConvertPubKeyToAddr, consAddr.String())
	}

	if voteExt.MinaAddress != pubKeyAddr {
		ctx.Logger().Info("validator address mismatch", "ext", voteExt.MinaAddress, "expected", pubKeyAddr)
		return errors.Wrap(types.ErrAddressMismatch, "validator"+voteExt.MinaAddress+pubKeyAddr)
	}

	err = verifySchnorr(voteExt, pubKey, ctx, *poseidonHash)
	if err != nil {
		return err
	}

	ctx.Logger().Info("signature verified", "validator", voteExt.MinaAddress)
	h.storeVote(uint64(req.GetHeight()), voteExt.MinaAddress, req.VoteExtension)
	return nil
}

// checks the validity of extension body for VerifyVoteExtensionHandler
func (h *VoteExtHandler) checkValidityOfVoteExtBody(ctx sdk.Context, req *abci.RequestVerifyVoteExtension, voteExt MinaSignatureVoteExt) error {
	extBody, err := h.getVoteExtBody(uint64(req.GetHeight()))
	if err != nil {
		ctx.Logger().Info("failed to get vote extension body", "error", err)
		return errors.Wrap(types.ErrFailedToGetVoteExtBody, "")
	}
	if !bytes.Equal(extBody.InitialValidatorSetRoot, voteExt.VoteExtBody.InitialValidatorSetRoot) {
		ctx.Logger().Info("initial validator set root mismatch", "ext", voteExt.VoteExtBody.InitialValidatorSetRoot, "expected", extBody.InitialValidatorSetRoot)
		return errors.Wrap(types.ErrValidatorSetRootMismatch, "")
	}

	if extBody.InitialBlockHeight != voteExt.VoteExtBody.InitialBlockHeight {
		ctx.Logger().Info("initial block height mismatch", "ext", voteExt.VoteExtBody.InitialBlockHeight, "expected", extBody.InitialBlockHeight)
		return errors.Wrap(types.ErrBlockHeightMismatch, "")
	}

	if !bytes.Equal(extBody.NewValidatorSetRoot, voteExt.VoteExtBody.NewValidatorSetRoot) {
		ctx.Logger().Info("new validator set root mismatch", "ext", voteExt.VoteExtBody.NewValidatorSetRoot, "expected", extBody.NewValidatorSetRoot)
		return errors.Wrap(types.ErrValidatorSetRootMismatch, "")
	}

	if extBody.NewBlockHeight != voteExt.VoteExtBody.NewBlockHeight {
		ctx.Logger().Info("new block height mismatch", "ext", voteExt.VoteExtBody.NewBlockHeight, "expected", extBody.NewBlockHeight)
		return errors.Wrap(types.ErrBlockHeightMismatch, "")
	}

	if !bytes.Equal(extBody.InitialStateRoot, voteExt.VoteExtBody.InitialStateRoot) {
		ctx.Logger().Info("initial state root mismatch", "ext", voteExt.VoteExtBody.InitialStateRoot, "expected", extBody.InitialStateRoot)
		return errors.Wrap(types.ErrStateRootMismatch, "initial")
	}

	if !bytes.Equal(extBody.NewStateRoot, voteExt.VoteExtBody.NewStateRoot) {
		ctx.Logger().Info("new state root mismatch", "ext", voteExt.VoteExtBody.NewStateRoot, "expected", extBody.NewStateRoot)
		return errors.Wrap(types.ErrStateRootMismatch, "new")
	}
	return nil
}
