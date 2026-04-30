package vote_ext

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (h *AbciHandler) PreBlocker() sdk.PreBlocker {
	return func(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {

		// If height is smaller than 4, we won't have any votes thus skip the proposal
		if req.GetHeight() < 3 {
			return &sdk.ResponsePreBlock{}, nil
		}

		err := h.votePersistenceKeeper.RemoveVotes(ctx)
		if err != nil {
			return nil, err
		}

		voteExtensionTx := req.Txs[0]

		voteExtensionTx = voteExtensionTx[len([]byte(VoteExtMarker)):]

		var pl payload

		err = json.Unmarshal(voteExtensionTx, &pl)
		if err != nil {
			return nil, err
		}

		currentValidatorSet, err := h.getHistoricalValidatorSet(ctx, req.GetHeight()-2)
		if err != nil {
			return nil, err
		}

		currentValidatorSetMap := make(map[string][]byte)

		for _, currentValidator := range currentValidatorSet {
			cosmosValidatorPublicKey, err := h.getValidatorPublicKey(ctx, currentValidator.ConsensusAddr)
			if err != nil {
				return nil, err
			}

			currentValidatorSetMap[hex.EncodeToString(cosmosValidatorPublicKey)] = cosmosValidatorPublicKey
		}

		for addr, vote := range pl.Votes {

			cosmosValidatorPublicKey, ok := currentValidatorSetMap[addr]
			if !ok {
				continue
			}

			exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, cosmosValidatorPublicKey)
			if err != nil {
				return nil, err
			}

			if !exists {
				return nil, fmt.Errorf("")
			}

			minaKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, cosmosValidatorPublicKey)
			if err != nil {
				return nil, err
			}

			err = h.votePersistenceKeeper.SetVote(ctx, req.GetHeight()-2, minaKey, vote)
			if err != nil {
				return nil, err
			}

		}

		return &sdk.ResponsePreBlock{}, nil
	}
}
