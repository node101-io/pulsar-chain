package abci

import (
	"bytes"
	"fmt"

	cometabci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// extractPayload decodes the reserved vote-extension payload from the first proposal tx.
// The found return value is false only when the reserved payload is absent; malformed
// marker payloads return found=true with an error so callers can distinguish absence
// from invalid protocol data.
func extractPayload(txs [][]byte) (Payload, bool, error) {

	if len(txs) == 0 {
		return Payload{}, false, nil
	}

	if !bytes.HasPrefix(txs[0], voteExtMarkerBytes) {
		return Payload{}, false, nil
	}

	voteExtensionTx := txs[0][len(voteExtMarkerBytes):]
	if len(voteExtensionTx) == 0 {
		return Payload{}, true, ErrInvalidPayload
	}

	var pl Payload

	err := pl.Unmarshal(voteExtensionTx)
	if err != nil {
		return Payload{}, true, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	return pl, true, nil
}

func validatePayloadHeight(pl Payload, expectedHeight int64) error {
	if pl.VoteExtensionHeight != expectedHeight {
		return fmt.Errorf("%w: expected %d, got %d", ErrInvalidPayloadHeight, expectedHeight, pl.VoteExtensionHeight)
	}

	return nil
}

// constructPayload derives the internal proposal payload from CometBFT's
// LocalLastCommit.Votes. CometBFT builds that list with one slot per validator
// index; duplicate handling for proposer-supplied payload bytes belongs in the
// payload validation path, not in this construction path.
func (h *ABCIHandler) constructPayload(ctx sdk.Context, proposalHeight int64, voteExtensions []cometabci.ExtendedVoteInfo) (Payload, error) {

	var voteExtsForGivenBlock []*PayloadVoteExtension

	currentValidatorSetMap := make(map[string]bool)
	// A proposal at height P carries the vote extensions produced by consensus
	// for height P-1. The payload height records that consensus production height,
	// not the proposal height and not the signed state height inside the body.
	voteExtensionHeight := proposalHeight - 1

	// The validator set used for payload eligibility must match the set that
	// produced the vote extensions at voteExtensionHeight.
	currentValidatorSet, err := h.getValidatorSet(ctx, voteExtensionHeight)
	if err != nil {
		return Payload{}, err
	}

	for _, currentValidator := range currentValidatorSet {

		consAddr, err := currentValidator.GetConsAddr()
		if err != nil {
			continue
		}

		currentValidatorSetMap[string(consAddr)] = true
	}

	for _, vote := range voteExtensions {

		if !currentValidatorSetMap[string(vote.Validator.Address)] {
			continue
		}

		if vote.BlockIdFlag != tmproto.BlockIDFlagCommit {
			continue
		}

		if len(vote.VoteExtension) == 0 {
			continue
		}

		cosmosValidatorPubKey, err := h.getConsPubKeyByConsAddr(ctx, vote.Validator.Address)
		if err != nil {
			return Payload{}, err
		}

		voteExtsForGivenBlock = append(voteExtsForGivenBlock, &PayloadVoteExtension{
			ConsensusPublicKey: cosmosValidatorPubKey,
			VoteExtension:      vote.VoteExtension,
		})
	}

	if len(voteExtsForGivenBlock) == 0 {
		return Payload{}, ErrNoVoteExtensionsForPayload
	}

	return Payload{VoteExtensionHeight: voteExtensionHeight, VoteExtensions: voteExtsForGivenBlock}, nil
}
