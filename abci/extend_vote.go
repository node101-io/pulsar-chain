package vote_ext

import (
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	votepersistence "github.com/node101-io/pulsar-chain/x/votepersistence/keeper"
)

const ActionsReducedRoot string = "pulsar"

type validatorInfo struct {
	ConsensusAddr []byte
	Power         int64
}

type VoteExtensionBody struct {
	NextValidatorSetHash []byte
	CurrentStateRoot     []byte
	CurrentBlockHeight   int64
}

type AbciHandler struct {
	secondaryKey          SecondaryKey
	stakingKeeper         stakingkeeper.Keeper
	keyregistryKeeper     keyregistrykeeper.Keeper
	votePersistenceKeeper votepersistence.Keeper
}

func (h *AbciHandler) ExtendVoteHandler() sdk.ExtendVoteHandler {
	return func(ctx sdk.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {

		if req.GetHeight() < 3 {
			return &abci.ResponseExtendVote{VoteExtension: []byte{}}, nil
		}

		body, err := h.constructVoteExtBody(ctx, req.GetHeight())
		if err != nil {
			return nil, err
		}

		bz := MockSign(body)

		return &abci.ResponseExtendVote{VoteExtension: bz}, nil
	}
}
