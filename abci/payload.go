package vote_ext

import (
	"bytes"
	"encoding/hex"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	votepersistenceTypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

func extractPayload(txs [][]byte) (Payload, error) {

	if len(txs) == 0 {
		return Payload{}, nil
	}

	if !bytes.HasPrefix(txs[0], []byte(VoteExtMarker)) {
		return Payload{}, votepersistenceTypes.ErrVoteExtMarkerNotFound
	}

	voteExtensionTx := txs[0][len([]byte(VoteExtMarker)):]

	var pl Payload

	err := pl.Unmarshal(voteExtensionTx)
	if err != nil {
		return Payload{}, err
	}
	return pl, nil
}

func (h *ABCIHandler) constructPayload(ctx sdk.Context, blockHeight int64, voteExtensions []abci.ExtendedVoteInfo) (Payload, error) {

	var voteExtsForGivenBlock []*Votes

	currentValidatorSetMap := make(map[string]bool)

	currentValidatorSet, err := h.getValidatorSet(ctx, blockHeight-2)
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

		cosmosValidatorPubKey, err := h.getValidatorPublicKey(ctx, vote.Validator.Address)
		if err != nil {
			return Payload{}, err
		}

		voteExtsForGivenBlock = append(voteExtsForGivenBlock, &Votes{
			ConsensusPublicKey: hex.EncodeToString(cosmosValidatorPubKey),
			VoteExtension:      vote.VoteExtension,
		})
	}

	if len(voteExtsForGivenBlock) == 0 {
		return Payload{}, nil
	}

	return Payload{Height: blockHeight - 1, Votes: voteExtsForGivenBlock}, nil
}
