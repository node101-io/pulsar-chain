package abci

import (
	"encoding/hex"
	"fmt"

	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (h *ABCIHandler) PreBlocker() sdk.PreBlocker {
	return func(ctx sdk.Context, req *cometabci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {

		// If height is smaller than 4, we won't have any votes thus skip the proposal
		if req.GetHeight() < 4 {
			return &sdk.ResponsePreBlock{}, nil
		}

		err := h.votePersistenceKeeper.Clear(ctx)
		if err != nil {
			return nil, err
		}

		pl, err := extractPayload(req.Txs)
		if err != nil {
			return nil, err
		}

		currentValidatorSet, err := h.getValidatorSet(ctx, req.GetHeight()-2)
		if err != nil {
			return nil, err
		}

		currentValidatorSetMap := make(map[string][]byte)

		for _, currentValidator := range currentValidatorSet {

			consAddr, err := currentValidator.GetConsAddr()
			if err != nil {
				continue
			}

			cosmosValidatorPublicKey, err := h.getValidatorPublicKey(ctx, consAddr)
			if err != nil {
				return nil, err
			}

			currentValidatorSetMap[hex.EncodeToString(cosmosValidatorPublicKey)] = cosmosValidatorPublicKey
		}

		for _, vote := range pl.Votes {

			cosmosValidatorPublicKey, ok := currentValidatorSetMap[vote.ConsensusPublicKey]
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

			err = h.votePersistenceKeeper.SetVote(ctx, req.GetHeight()-2, minaKey, vote.VoteExtension)
			if err != nil {
				return nil, err
			}

		}

		return &sdk.ResponsePreBlock{}, nil
	}
}
