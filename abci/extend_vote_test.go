package abci

import (
	"testing"

	cometabci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestExtendVoteHandlerDisabledHeight(t *testing.T) {
	ctx := sdk.Context{}.WithConsensusParams(tmproto.ConsensusParams{
		Abci: &tmproto.ABCIParams{VoteExtensionsEnableHeight: 1},
	})
	handler := (&ABCIHandler{}).ExtendVoteHandler()

	response, err := handler(ctx, &cometabci.RequestExtendVote{Height: 3})

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Empty(t, response.VoteExtension)
}

func TestExtendVoteHandlerConsensusParamsFail(t *testing.T) {
	ctx := sdk.Context{}.WithConsensusParams(tmproto.ConsensusParams{})
	handler := (&ABCIHandler{}).ExtendVoteHandler()

	response, err := handler(ctx, &cometabci.RequestExtendVote{Height: 4})

	require.Nil(t, response)
	require.ErrorIs(t, err, ErrUnableToReadConsensusParams)
}
