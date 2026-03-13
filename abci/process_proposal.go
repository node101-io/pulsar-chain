package abci

import (
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/mina-signer-go/signature"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

// ProcessProposalHandler now enforces a lighter rule: each validator only
// checks whether _its own_ vote-extension for the previous height is included
// in the block proposal. If the validator's signature is missing, the proposal
// is rejected. This shifts the ≥⅔ voting-power requirement to CometBFT itself:
// a block that does not include ≥⅔ of the network's vote-extensions will be
// rejected automatically because fewer than ⅔ of validators will `ACCEPT` it.
func (h *VoteExtHandler) ProcessProposalHandler() sdk.ProcessProposalHandler {
	type payload struct {
		Height uint64            `json:"height"`
		Votes  map[string][]byte `json:"votes"`
	}

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

		marker := []byte("VOTEEXT:")
		txs := req.GetTxs()

		// If the special transaction is missing, immediately reject.
		if len(txs) == 0 || len(txs[0]) <= len(marker) || string(txs[0][:len(marker)]) != string(marker) {
			ctx.Logger().Info("Proposal missing VOTEEXT transaction", "looking for height", targetHeight, "proposal height", req.GetHeight())
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, fmt.Errorf("proposal missing VOTEEXT transaction")
		}

		// Decode the payload containing vote-extensions.
		var data payload
		if err := json.Unmarshal(txs[0][len(marker):], &data); err != nil {
			ctx.Logger().Info("Malformed VOTEEXT payload", "error", err)
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, fmt.Errorf("malformed VOTEEXT payload: %w", err)
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
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, fmt.Errorf("failed to convert public key to address: %w", err)
		}

		// Find our address in the map.
		extBz, ok := data.Votes[myAddr]
		if !ok {
			ctx.Logger().Info("Validator's vote extension missing from proposal", "validator", myAddr)
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, fmt.Errorf("validator's vote extension missing from proposal")
		}

		var ve MinaSignatureVoteExt
		if err := json.Unmarshal(extBz, &ve); err != nil {
			ctx.Logger().Info("Malformed vote extension entry", "error", err)
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, fmt.Errorf("malformed vote extension entry: %w", err)
		}

		// Initialize poseidon hash
		poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

		extBodyHashInput := ve.VoteExtBody.GetPoseidonHashInput(ctx, poseidonHash)

		// Verify signature
		sig := new(signature.Signature)
		if err := sig.UnmarshalBytes(ve.Signature); err != nil {
			ctx.Logger().Info("Invalid signature encoding", "error", err)
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, fmt.Errorf("invalid signature encoding: %w", err)
		}

		pubKey := h.MinaPrivateKey.PublicKey
		if !pubKey.Verify(sig, extBodyHashInput, types.DevnetNetworkID) {
			ctx.Logger().Info("Signature verification failed", "error", err)
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, fmt.Errorf("signature verification failed: %w", err)
		}

		// Vote extension successfully verified
		return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
	}
}
