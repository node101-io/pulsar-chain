package vote_ext

import (
	"bytes"
	"encoding/json"

	"strconv"

	"cosmossdk.io/errors"
	abci "github.com/cometbft/cometbft/abci/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

func (h *VoteExtHandler) VerifyVoteExtensionHandler() sdk.VerifyVoteExtensionHandler {
	return func(ctx sdk.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
		// Unmarshal the extension payload
		var voteExt MinaSignatureVoteExt
		if err := json.Unmarshal(req.VoteExtension, &voteExt); err != nil {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, errors.Wrap(types.ErrMalformedVoteExtPayload, "")
		}

		err := h.verifyExtensionSig(ctx, req, voteExt)
		if err != nil {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, err
		}

		err = h.checkValidityOfVoteExtBody(ctx, req, voteExt)
		if err != nil {
			return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, err
		}

		return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_ACCEPT}, nil
	}
}

func (h *VoteExtHandler) verifyExtensionSig(ctx sdk.Context, req *abci.RequestVerifyVoteExtension, voteExt MinaSignatureVoteExt) error {

	// Get the validator address from the request
	consAddr := sdk.ConsAddress(req.ValidatorAddress)

	consensusPubKey, err := h.stakingKeeper.GetPubKeyByConsAddr(ctx, consAddr)
	if err != nil {
		return errors.Wrap(types.ErrUnknownValidator, consAddr.String())
	}

	pubKey, err := cryptocodec.FromCmtProtoPublicKey(consensusPubKey)
	if err != nil {
		return errors.Wrap(types.ErrFailedToUnmarshal, consAddr.String())
	}

	exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, pubKey.Bytes())
	if err != nil {
		return errors.Wrap(types.ErrInternal, "Cosmos to Mina key lookup failed for validator "+consAddr.String())
	}

	// Initialize poseidon hash
	poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

	minaPubKey := new(keys.PublicKey)
	if exists {
		minaPublicKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, pubKey.Bytes())
		if err != nil {
			return errors.Wrap(types.ErrInternal, "Mina public key lookup failed for validator "+consAddr.String())
		}
		err = minaPubKey.UnmarshalBytes(minaPublicKey)
		if err != nil {
			return errors.Wrap(types.ErrFailedToUnmarshal, consAddr.String())
		}
	} else {
		// Testnet-only fallback: trust the Mina address declared in the vote
		// extension when the registry mapping is missing.
		fallbackPubKey, err := new(keys.PublicKey).FromAddress(voteExt.MinaAddress)
		if err != nil {
			return errors.Wrap(types.ErrUnknownValidator, consAddr.String())
		}
		*minaPubKey = fallbackPubKey
	}

	// Check if the address is correct.
	pubKeyAddr, err := minaPubKey.ToAddress()
	if err != nil {
		return errors.Wrap(types.ErrFailedToConvertPubKeyToAddr, consAddr.String())
	}

	if voteExt.MinaAddress != pubKeyAddr {
		return errors.Wrap(types.ErrAddressMismatch, "validator"+voteExt.MinaAddress+pubKeyAddr)
	}

	err = verifySchnorr(voteExt, *minaPubKey, ctx, *poseidonHash)
	if err != nil {
		return err
	}

	h.storeVote(uint64(req.GetHeight()), voteExt.MinaAddress, req.VoteExtension)
	return nil
}

// checks the validity of extension body for VerifyVoteExtensionHandler
func (h *VoteExtHandler) checkValidityOfVoteExtBody(ctx sdk.Context, req *abci.RequestVerifyVoteExtension, voteExt MinaSignatureVoteExt) error {
	extBody, err := h.getVoteExtBody(uint64(req.GetHeight()))
	if err != nil {
		return errors.Wrap(types.ErrFailedToGetVoteExtBody, "failed to get vote extension body for height "+strconv.FormatUint(uint64(req.GetHeight()), 10))
	}
	if !bytes.Equal(extBody.InitialValidatorSetRoot, voteExt.VoteExtBody.InitialValidatorSetRoot) {
		return errors.Wrap(types.ErrValidatorSetRootMismatch, "initial validator set root mismatch for height "+strconv.FormatUint(uint64(req.GetHeight()), 10))
	}

	if extBody.InitialBlockHeight != voteExt.VoteExtBody.InitialBlockHeight {
		return errors.Wrap(types.ErrBlockHeightMismatch, "initial block height mismatch for height "+strconv.FormatUint(uint64(req.GetHeight()), 10))
	}

	if !bytes.Equal(extBody.NewValidatorSetRoot, voteExt.VoteExtBody.NewValidatorSetRoot) {
		return errors.Wrap(types.ErrValidatorSetRootMismatch, "new validator set root mismatch for height "+strconv.FormatUint(uint64(req.GetHeight()), 10))
	}

	if extBody.NewBlockHeight != voteExt.VoteExtBody.NewBlockHeight {
		return errors.Wrap(types.ErrBlockHeightMismatch, "new block height mismatch for height "+strconv.FormatUint(uint64(req.GetHeight()), 10))
	}

	if !bytes.Equal(extBody.InitialStateRoot, voteExt.VoteExtBody.InitialStateRoot) {
		return errors.Wrap(types.ErrStateRootMismatch, "initial state root mismatch for height "+strconv.FormatUint(uint64(req.GetHeight()), 10))
	}

	if !bytes.Equal(extBody.NewStateRoot, voteExt.VoteExtBody.NewStateRoot) {
		return errors.Wrap(types.ErrStateRootMismatch, "new state root mismatch for height "+strconv.FormatUint(uint64(req.GetHeight()), 10))
	}
	return nil
}
