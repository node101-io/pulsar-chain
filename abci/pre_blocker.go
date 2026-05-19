package abci

import (
	"fmt"

	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (h *ABCIHandler) PreBlocker() sdk.PreBlocker {
	return func(ctx sdk.Context, req *cometabci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {

		shouldPersistVoteExtensions, err := shouldRequireProposalPayloadAtHeight(ctx, req.GetHeight())
		if err != nil {
			return nil, err
		}
		if !shouldPersistVoteExtensions {
			return &sdk.ResponsePreBlock{}, nil
		}

		if err := h.votePersistenceKeeper.Clear(ctx); err != nil {
			return nil, err
		}

		pl, payloadFound, err := extractPayload(req.Txs)
		if err != nil {
			return nil, err
		}
		if !payloadFound {
			return nil, ErrVoteExtPayloadNotFound
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

			cosmosValidatorPublicKey, err := h.getConsPubKeyByConsAddr(ctx, consAddr)
			if err != nil {
				return nil, err
			}

			currentValidatorSetMap[string(cosmosValidatorPublicKey)] = cosmosValidatorPublicKey
		}

		for _, vote := range pl.Votes {

			cosmosValidatorPublicKey, ok := currentValidatorSetMap[string(vote.ConsensusPublicKey)]
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
