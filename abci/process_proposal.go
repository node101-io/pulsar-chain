package vote_ext

import (
	"encoding/json"

	"cosmossdk.io/errors"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

type payload struct {
	Height uint64            `json:"height"`
	Votes  map[string][]byte `json:"votes"`
}

// reconstructs vote extension body for process proposal
func (h *VoteExtHandler) reconstructVoteExtBody(ctx sdk.Context, req *abci.RequestProcessProposal) (MinaSignatureVoteExt, error) {

	targetHeight := uint64(req.GetHeight() - 1)

	txs := req.GetTxs()

	// If the special transaction is missing, immediately reject.
	if len(txs) == 0 || len(txs[0]) <= len(types.VoteExtMarker) || string(txs[0][:len(types.VoteExtMarker)]) != string(types.VoteExtMarker) {
		ctx.Logger().Info("Proposal missing VOTEEXT transaction", "looking for height", targetHeight, "proposal height", req.GetHeight())
		return MinaSignatureVoteExt{}, errors.Wrap(types.ErrMissingVoteExt, "")
	}

	// Decode the payload containing vote-extensions.
	var data payload
	if err := json.Unmarshal(txs[0][len(types.VoteExtMarker):], &data); err != nil {
		ctx.Logger().Info("Malformed VOTEEXT payload", "error", err)
		return MinaSignatureVoteExt{}, errors.Wrap(types.ErrMalformedVoteExtPayload, "")
	}

	// Set the votes in the payload if we don't have them in our map
	for _, voteBytes := range data.Votes {
		var ve MinaSignatureVoteExt
		if err := json.Unmarshal(voteBytes, &ve); err != nil {
			continue // skip malformed entry
		}
		h.storeVote(uint64(req.GetHeight()), ve.MinaAddress, voteBytes)
	}

	// Create our address from our local Mina public key.
	myAddr, err := h.MinaPrivateKey.PublicKey.ToAddress()
	if err != nil {
		ctx.Logger().Info("Failed to convert public key to address", "error", err)
		return MinaSignatureVoteExt{}, errors.Wrap(types.ErrFailedToConvertPubKeyToAddr, "")
	}

	// Find our address in the map.
	extBz, ok := data.Votes[myAddr]
	if !ok {
		ctx.Logger().Info("Validator's vote extension missing from proposal", "validator", myAddr)
		return MinaSignatureVoteExt{}, errors.Wrap(types.ErrValidatorVoteExtMissing, "")
	}

	var ve MinaSignatureVoteExt
	if err := json.Unmarshal(extBz, &ve); err != nil {
		ctx.Logger().Info("Malformed vote extension entry", "error", err)
		return MinaSignatureVoteExt{}, errors.Wrap(types.ErrMalformedVoteExtEntry, "")
	}
	return ve, nil
}

// ProcessProposalHandler now enforces a lighter rule: each validator only
// checks whether _its own_ vote-extension for the previous height is included
// in the block proposal. If the validator's signature is missing, the proposal
// is rejected. This shifts the ≥⅔ voting-power requirement to CometBFT itself:
// a block that does not include ≥⅔ of the network's vote-extensions will be
// rejected automatically because fewer than ⅔ of validators will `ACCEPT` it.
func (h *VoteExtHandler) ProcessProposalHandler() sdk.ProcessProposalHandler {

	return func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		ctx.Logger().Info("ProcessProposalHandler:start", "height", req.GetHeight())
		// If height is 1, we won't have any votes thus skip the proposal
		if req.GetHeight() == 1 {
			ctx.Logger().Info("Height is 1, accepting proposal", "height", req.GetHeight())
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
		}
		targetHeight := uint64(req.GetHeight() - 1)

		// If the previous height has no votes, we don't wait for the VOTEEXT tx.
		if len(h.fetchVotes(targetHeight)) == 0 {
			ctx.Logger().Info("No votes for previous height, accepting proposal", "looking for height", targetHeight, "proposal height", req.GetHeight())
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
		}

		ve, err := h.reconstructVoteExtBody(ctx, req)
		if err != nil {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, err
		}
		// Initialize poseidon hash
		poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

		err = verifySchnorr(ve, *h.MinaPrivateKey.PublicKey, ctx, *poseidonHash)
		if err != nil {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, err
		}

		// Vote extension successfully verified
		return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
	}
}
