package abci

import (
	"bytes"
	"fmt"
	"sort"

	cometabci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// extractPayload decodes the reserved vote-extension payload from the first
// proposal transaction. The fixed first position makes the consensus-internal
// data unambiguous and prevents a user transaction from being interpreted as a
// second payload. found is false only when the marker is absent; a present but
// malformed marker is reported separately so ProcessProposal rejects it rather
// than silently treating invalid protocol data as optional.
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
// LocalLastCommit.Votes. The mandatory section preserves the Mina transition
// signatures used by the existing bridge flow. The optional section retains a
// validator's complete signed envelope only when it contains a verification
// action that is valid against the proposer's current state. CometBFT builds the
// input with one slot per validator index; duplicate handling for proposer-made
// payload bytes therefore belongs in ProcessProposal, not this local path.
func (h *ABCIHandler) constructPayload(ctx sdk.Context, proposalHeight int64, round int32, voteExtensions []cometabci.ExtendedVoteInfo) (Payload, error) {

	var voteExtsForGivenBlock []*PayloadVoteExtension
	var verificationEntries []*PayloadVerificationEntry

	// A proposal at height P carries the vote extensions produced by consensus
	// for height P-1. The payload height records that consensus production height,
	// not the proposal height and not the signed state height inside the body.
	voteExtensionHeight := proposalHeight - 1

	// The validator set used for payload eligibility must match the set that
	// produced the vote extensions at voteExtensionHeight. Looking at the latest
	// set would make key rotation or membership changes reinterpret historical
	// votes differently on different nodes.
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

		// The compact mandatory list carries only the Mina transition signature so
		// it remains available even if verification is disabled or malformed. The
		// complete CometBFT-signed envelope is retained separately only when it has
		// a usable verification action; that outer signature later authenticates
		// the commitment or revelation as this validator's action.
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

	// Canonical validator-address ordering makes proposal payload bytes
	// deterministic regardless of LocalLastCommit iteration order. It also lets
	// validators reject duplicates and ambiguous order with a single linear pass.
	sort.Slice(verificationEntries, func(i, j int) bool {
		return bytes.Compare(verificationEntries[i].ValidatorAddress, verificationEntries[j].ValidatorAddress) < 0
	})

	return Payload{
		VoteExtensionHeight: voteExtensionHeight,
		VoteExtensions:      voteExtsForGivenBlock,
		VerificationEntries: verificationEntries,
	}, nil
}
