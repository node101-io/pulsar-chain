package abci

import (
	"testing"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestFirstPulsarVoteExtensionHeight(t *testing.T) {
	tests := []struct {
		name         string
		enableHeight int64
		expected     int64
	}{
		{name: "disabled", enableHeight: 0, expected: 0},
		{name: "enable at first height", enableHeight: 1, expected: 2},
		{name: "enable when proof first becomes available", enableHeight: 2, expected: 2},
		{name: "enable after proof is available", enableHeight: 3, expected: 3},
		{name: "enable after proof is already available", enableHeight: 7, expected: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, firstPulsarVoteExtensionHeight(tt.enableHeight))
		})
	}
}

func TestVoteExtensionHeightPolicyDisabled(t *testing.T) {
	ctx := voteExtensionPolicyTestContext(0)

	for _, height := range []int64{1, 3, 10} {
		shouldExtend, err := shouldExtendVoteAtHeight(ctx, height)
		require.NoError(t, err)
		require.False(t, shouldExtend)

		shouldRequirePayload, err := shouldRequireProposalPayloadAtHeight(ctx, height)
		require.NoError(t, err)
		require.False(t, shouldRequirePayload)
	}
}

func TestVoteExtensionHeightPolicyEnableAtGenesis(t *testing.T) {
	ctx := voteExtensionPolicyTestContext(1)

	shouldExtend, err := shouldExtendVoteAtHeight(ctx, 1)
	require.NoError(t, err)
	require.False(t, shouldExtend)

	shouldExtend, err = shouldExtendVoteAtHeight(ctx, 2)
	require.NoError(t, err)
	require.True(t, shouldExtend)

	shouldRequirePayload, err := shouldRequireProposalPayloadAtHeight(ctx, 2)
	require.NoError(t, err)
	require.False(t, shouldRequirePayload)

	shouldRequirePayload, err = shouldRequireProposalPayloadAtHeight(ctx, 3)
	require.NoError(t, err)
	require.True(t, shouldRequirePayload)
}

func TestVoteExtensionHeightPolicyEnableAfterProofIsAvailable(t *testing.T) {
	ctx := voteExtensionPolicyTestContext(7)

	shouldExtend, err := shouldExtendVoteAtHeight(ctx, 6)
	require.NoError(t, err)
	require.False(t, shouldExtend)

	shouldExtend, err = shouldExtendVoteAtHeight(ctx, 7)
	require.NoError(t, err)
	require.True(t, shouldExtend)

	shouldRequirePayload, err := shouldRequireProposalPayloadAtHeight(ctx, 7)
	require.NoError(t, err)
	require.False(t, shouldRequirePayload)

	shouldRequirePayload, err = shouldRequireProposalPayloadAtHeight(ctx, 8)
	require.NoError(t, err)
	require.True(t, shouldRequirePayload)
}

func TestVoteExtensionHeightPolicyReturnsConsensusParamsError(t *testing.T) {
	ctx := sdk.Context{}.WithConsensusParams(tmproto.ConsensusParams{})

	shouldExtend, err := shouldExtendVoteAtHeight(ctx, 3)
	require.False(t, shouldExtend)
	require.ErrorIs(t, err, ErrUnableToReadConsensusParams)

	shouldRequirePayload, err := shouldRequireProposalPayloadAtHeight(ctx, 4)
	require.False(t, shouldRequirePayload)
	require.ErrorIs(t, err, ErrUnableToReadConsensusParams)
}

func voteExtensionPolicyTestContext(voteExtensionsEnableHeight int64) sdk.Context {
	return sdk.Context{}.WithConsensusParams(tmproto.ConsensusParams{
		Abci: &tmproto.ABCIParams{VoteExtensionsEnableHeight: voteExtensionsEnableHeight},
	})
}
