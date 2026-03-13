package vote_ext

import (
	"bytes"
	"encoding/json"
	"fmt"

	"cosmossdk.io/errors"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/mina-signer-go/signature"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

func (h *VoteExtHandler) VerifyVoteExtensionHandler() sdk.VerifyVoteExtensionHandler {
	return func(ctx sdk.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
		ctx.Logger().Info("VerifyVoteExtensionHandler:start", "height", req.GetHeight())
		// Unmarshal the extension payload
		var voteExt MinaSignatureVoteExt
		if err := json.Unmarshal(req.VoteExtension, &voteExt); err != nil {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, fmt.Errorf("invalid vote extension payload: %w", err)
		}

		// Log incoming vote extension for visibility
		ctx.Logger().Info("incoming vote extension", "voteExt", voteExt)

		// Get the validator address from the request
		consAddr := sdk.ConsAddress(req.ValidatorAddress)

		// Log incoming vote-extension for visibility
		ctx.Logger().Info("VerifyVoteExtension", "height", req.GetHeight(), "validator", consAddr.String())

		exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, consAddr.Bytes())
		if err != nil {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, fmt.Errorf("internal error")
		}

		// unknown validator – ignore the vote.
		if !exists {
			ctx.Logger().Info("unknown validator", consAddr.String())
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, fmt.Errorf("unknown validator address %s", consAddr.String())
		}

		minaPublicKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, consAddr.Bytes())
		if err != nil {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, fmt.Errorf("internal error")
		}

		pubKey, err := new(keys.PublicKey).FromAddress(string(minaPublicKey))
		if err != nil {
			ctx.Logger().Info("failed to unmarshal mina public key for validator", "validator", consAddr.String(), "error", err)
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, errors.Wrap(types.ErrFailedToUnmarshal, consAddr.String())
		}

		sig := new(signature.Signature)
		if err := sig.UnmarshalBytes(voteExt.Signature); err != nil {
			ctx.Logger().Info("invalid signature encoding", "error", err)
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, errors.Wrap(types.ErrInvalidSigEncoding, "")
		}

		// Check if the address is correct.
		pubKeyAddr, err := pubKey.ToAddress()
		if err != nil {
			ctx.Logger().Info("failed to convert public key to address", "validator", consAddr.String(), "error", err)
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, fmt.Errorf("failed to convert public key to address for validator %s: %w", consAddr.String(), err)
		}

		if voteExt.MinaAddress != pubKeyAddr {
			ctx.Logger().Info("validator address mismatch", "ext", voteExt.MinaAddress, "expected", pubKeyAddr)
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, errors.Wrap(types.ErrAddressMismatch, "validator"+voteExt.MinaAddress+pubKeyAddr)
		}

		// Initialize poseidon hash
		poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

		extBodyHashInput := voteExt.VoteExtBody.GetPoseidonHashInput(ctx, poseidonHash)

		// Verify signature; if ok, keep the vote in memory.
		if pubKey.Verify(sig, extBodyHashInput, types.DevnetNetworkID) {
			ctx.Logger().Info("signature verified", "validator", voteExt.MinaAddress)
			h.storeVote(uint64(req.GetHeight()), voteExt.MinaAddress, req.VoteExtension)
		}

		extBody, err := h.getVoteExtBody(uint64(req.GetHeight()))
		if err != nil {
			ctx.Logger().Info("failed to get vote extension body", "error", err)
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, errors.Wrap(types.ErrFailedToGetVoteExtBody, "")
		}

		if !bytes.Equal(extBody.InitialValidatorSetRoot, voteExt.VoteExtBody.InitialValidatorSetRoot) {
			ctx.Logger().Info("initial validator set root mismatch", "ext", voteExt.VoteExtBody.InitialValidatorSetRoot, "expected", extBody.InitialValidatorSetRoot)
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, errors.Wrap(types.ErrValidatorSetRootMismatch, "")
		}

		if extBody.InitialBlockHeight != voteExt.VoteExtBody.InitialBlockHeight {
			ctx.Logger().Info("initial block height mismatch", "ext", voteExt.VoteExtBody.InitialBlockHeight, "expected", extBody.InitialBlockHeight)
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, errors.Wrap(types.ErrBlockHeightMismatch, "")
		}

		if !bytes.Equal(extBody.NewValidatorSetRoot, voteExt.VoteExtBody.NewValidatorSetRoot) {
			ctx.Logger().Info("new validator set root mismatch", "ext", voteExt.VoteExtBody.NewValidatorSetRoot, "expected", extBody.NewValidatorSetRoot)
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, errors.Wrap(types.ErrValidatorSetRootMismatch, "")
		}

		if extBody.NewBlockHeight != voteExt.VoteExtBody.NewBlockHeight {
			ctx.Logger().Info("new block height mismatch", "ext", voteExt.VoteExtBody.NewBlockHeight, "expected", extBody.NewBlockHeight)
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, errors.Wrap(types.ErrBlockHeightMismatch, "")
		}

		if !bytes.Equal(extBody.InitialStateRoot, voteExt.VoteExtBody.InitialStateRoot) {
			ctx.Logger().Info("initial state root mismatch", "ext", voteExt.VoteExtBody.InitialStateRoot, "expected", extBody.InitialStateRoot)
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, errors.Wrap(types.ErrStateRootMismatch, "initial")
		}

		if !bytes.Equal(extBody.NewStateRoot, voteExt.VoteExtBody.NewStateRoot) {
			ctx.Logger().Info("new state root mismatch", "ext", voteExt.VoteExtBody.NewStateRoot, "expected", extBody.NewStateRoot)
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, errors.Wrap(types.ErrStateRootMismatch, "new")
		}

		ctx.Logger().Info("vote extension verified", "validator", voteExt.MinaAddress)
		return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_ACCEPT}, nil
	}
}
