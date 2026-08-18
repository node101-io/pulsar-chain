package abci

import (
	"bytes"
	"fmt"
	"sort"

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
	for _, tx := range txs[1:] {
		if bytes.HasPrefix(tx, voteExtMarkerBytes) {
			return Payload{}, true, ErrInvalidPayload
		}
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
func (h *ABCIHandler) constructPayload(ctx sdk.Context, proposalHeight int64, round int32, voteExtensions []cometabci.ExtendedVoteInfo) (Payload, error) {

	var voteExtsForGivenBlock []*PayloadVoteExtension
	var verificationEntries []*PayloadVerificationEntry

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
	currentValidatorSetMap, err := validatorSetByConsensusAddress(currentValidatorSet)
	if err != nil {
		return Payload{}, err
	}

	for _, vote := range voteExtensions {

		validator, eligible := currentValidatorSetMap[string(vote.Validator.Address)]
		if !eligible {
			continue
		}

		if vote.BlockIdFlag != tmproto.BlockIDFlagCommit {
			continue
		}

		if len(vote.VoteExtension) == 0 {
			continue
		}

		composite, err := decodeCompositeVoteExtension(vote.VoteExtension)
		if err != nil {
			continue
		}
		consensusPublicKey, err := validator.ConsPubKey()
		if err != nil {
			return Payload{}, err
		}
		cosmosValidatorPubKey := consensusPublicKey.Bytes()

		voteExtsForGivenBlock = append(voteExtsForGivenBlock, &PayloadVoteExtension{
			ConsensusPublicKey: cosmosValidatorPubKey,
			VoteExtension:      composite.TransitionSignature,
		})

		verificationPayload := composite.VerificationPayload
		if verificationPayload == nil || h.verificationKeeper == nil {
			continue
		}
		if err := validateVerificationPayloadStructure(verificationPayload, uint64(proposalHeight)); err != nil {
			continue
		}
		operator, err := sdk.ValAddressFromBech32(validator.GetOperator())
		if err != nil {
			continue
		}
		if err := h.verificationKeeper.ValidateVerificationPayload(
			ctx, operator, uint64(proposalHeight), verificationPayload.Commitment, verificationPayload.Revelations,
		); err != nil {
			continue
		}
		verificationEntries = append(verificationEntries, &PayloadVerificationEntry{
			ValidatorAddress:       append([]byte(nil), vote.Validator.Address...),
			SourceHeight:           voteExtensionHeight,
			Round:                  round,
			CompositeVoteExtension: append([]byte(nil), vote.VoteExtension...),
			ExtensionSignature:     append([]byte(nil), vote.ExtensionSignature...),
		})
	}

	if len(voteExtsForGivenBlock) == 0 {
		return Payload{}, ErrNoVoteExtensionsForPayload
	}

	sort.Slice(verificationEntries, func(i, j int) bool {
		return bytes.Compare(verificationEntries[i].ValidatorAddress, verificationEntries[j].ValidatorAddress) < 0
	})

	return Payload{
		VoteExtensionHeight: voteExtensionHeight,
		VoteExtensions:      voteExtsForGivenBlock,
		VerificationEntries: verificationEntries,
	}, nil
}
