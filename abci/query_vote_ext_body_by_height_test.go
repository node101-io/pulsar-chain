package abci

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestVoteExtBodyByHeightRejectsNilRequest(t *testing.T) {
	handler := &ABCIHandler{}

	response, err := handler.VoteExtBodyByHeight(context.Background(), nil)

	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestVoteExtBodyByHeightRejectsHeightBelowMinimum(t *testing.T) {
	handler := &ABCIHandler{}

	response, err := handler.VoteExtBodyByHeight(
		sdk.Context{}.WithBlockHeight(10),
		&QueryVoteExtBodyByHeightRequest{VoteExtensionHeight: MinPulsarVoteExtensionHeight - 1},
	)

	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestVoteExtBodyByHeightRejectsCurrentOrFutureHeight(t *testing.T) {
	handler := &ABCIHandler{}

	response, err := handler.VoteExtBodyByHeight(
		sdk.Context{}.WithBlockHeight(10),
		&QueryVoteExtBodyByHeightRequest{VoteExtensionHeight: 10},
	)

	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestVoteExtBodyByHeightReturnsBodyForMinimumHeight(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	handler := newVoteExtBodyQueryTestHandler(t, []stakingtypes.Validator{validator}, map[string][]byte{
		string(consensusPubKeyBytes(t, validator)): testMinaPublicKey(t, [32]byte{1}),
	})

	response, err := handler.VoteExtBodyByHeight(
		sdk.Context{}.WithBlockHeight(3),
		&QueryVoteExtBodyByHeightRequest{VoteExtensionHeight: 2},
	)

	require.NoError(t, err)
	require.Equal(t, testActionsReducedRoot(), response.GetVoteExtBody().GetActionsReducedRoot())
	require.NotNil(t, response.GetVoteExtBody())
	require.Equal(t, int64(0), response.GetVoteExtBody().GetCurrentBlockHeight())
	require.Equal(t, testStateRoot32(), response.GetVoteExtBody().GetCurrentStateRoot())
	require.NotEmpty(t, response.GetVoteExtBody().GetNextValidatorSetHash())
}

func TestVoteExtBodyByHeightReturnsBodyForLaterHeight(t *testing.T) {
	voteExtensionHeight := int64(8)
	validator := newTestBondedValidator(t, 10)
	handler := newVoteExtBodyQueryTestHandler(t, []stakingtypes.Validator{validator}, map[string][]byte{
		string(consensusPubKeyBytes(t, validator)): testMinaPublicKey(t, [32]byte{1}),
	})

	response, err := handler.VoteExtBodyByHeight(
		sdk.Context{}.WithBlockHeight(10),
		&QueryVoteExtBodyByHeightRequest{VoteExtensionHeight: voteExtensionHeight},
	)

	require.NoError(t, err)
	require.Equal(t, testActionsReducedRoot(), response.GetVoteExtBody().GetActionsReducedRoot())
	require.NotNil(t, response.GetVoteExtBody())
	require.Equal(t, voteExtensionHeight-2, response.GetVoteExtBody().GetCurrentBlockHeight())
	require.Equal(t, testStateRoot32(), response.GetVoteExtBody().GetCurrentStateRoot())
	require.NotEmpty(t, response.GetVoteExtBody().GetNextValidatorSetHash())
}

func TestVoteExtBodyByHeightUsesExactSignedStateHeightForActionRoot(t *testing.T) {
	voteExtensionHeight := int64(8)
	signedStateHeight := voteExtensionHeight - 2
	validator := newTestBondedValidator(t, 10)

	handler := newVoteExtBodyQueryTestHandler(t, []stakingtypes.Validator{validator}, map[string][]byte{
		string(consensusPubKeyBytes(t, validator)): testMinaPublicKey(t, [32]byte{1}),
	})

	earlierRoot := mustReduceToFieldBytes([]byte("signed-state-height-5-root"))
	exactRoot := mustReduceToFieldBytes([]byte("signed-state-height-6-root"))
	laterRoot := mustReduceToFieldBytes([]byte("signed-state-height-7-root"))
	currentHeightRoot := mustReduceToFieldBytes([]byte("query-height-10-root"))

	handler.bridgeKeeper = testBridgeKeeper{
		rootsByHeight: map[int64][]byte{
			0:                     testActionsReducedRoot(),
			signedStateHeight - 1: earlierRoot,
			signedStateHeight:     exactRoot,
			signedStateHeight + 1: laterRoot,
			10:                    currentHeightRoot,
		},
	}

	response, err := handler.VoteExtBodyByHeight(
		sdk.Context{}.WithBlockHeight(10),
		&QueryVoteExtBodyByHeightRequest{VoteExtensionHeight: voteExtensionHeight},
	)

	require.NoError(t, err)
	require.NotNil(t, response.GetVoteExtBody())
	require.Equal(t, signedStateHeight, response.GetVoteExtBody().GetCurrentBlockHeight())
	require.Equal(t, exactRoot, response.GetVoteExtBody().GetActionsReducedRoot())
	require.NotEqual(t, earlierRoot, response.GetVoteExtBody().GetActionsReducedRoot())
	require.NotEqual(t, laterRoot, response.GetVoteExtBody().GetActionsReducedRoot())
	require.NotEqual(t, currentHeightRoot, response.GetVoteExtBody().GetActionsReducedRoot())
}

func TestVoteExtBodyByHeightReturnsHistoricalActionRoot(t *testing.T) {
	voteExtensionHeight := int64(8)
	signedStateHeight := voteExtensionHeight - 2
	validator := newTestBondedValidator(t, 10)

	handler := newVoteExtBodyQueryTestHandler(t, []stakingtypes.Validator{validator}, map[string][]byte{
		string(consensusPubKeyBytes(t, validator)): testMinaPublicKey(t, [32]byte{1}),
	})

	historicalRoot := mustReduceToFieldBytes([]byte("signed-state-height-6-root"))
	handler.bridgeKeeper = testBridgeKeeper{
		rootsByHeight: map[int64][]byte{
			0:                 testActionsReducedRoot(),
			signedStateHeight: historicalRoot,
		},
	}

	response, err := handler.VoteExtBodyByHeight(
		sdk.Context{}.WithBlockHeight(10),
		&QueryVoteExtBodyByHeightRequest{VoteExtensionHeight: voteExtensionHeight},
	)

	require.NoError(t, err)
	require.NotNil(t, response.GetVoteExtBody())
	require.Equal(t, historicalRoot, response.GetVoteExtBody().GetActionsReducedRoot())
}

func TestVoteExtBodyByHeightReturnsNotFoundForMissingMinaKey(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	handler := newVoteExtBodyQueryTestHandler(t, []stakingtypes.Validator{validator}, nil)

	response, err := handler.VoteExtBodyByHeight(
		sdk.Context{}.WithBlockHeight(10),
		&QueryVoteExtBodyByHeightRequest{VoteExtensionHeight: 8},
	)

	require.Nil(t, response)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestVoteExtBodyByHeightReturnsInternalForMalformedMinaKey(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	handler := newVoteExtBodyQueryTestHandler(t, []stakingtypes.Validator{validator}, map[string][]byte{
		string(consensusPubKeyBytes(t, validator)): []byte("malformed-mina-key"),
	})

	response, err := handler.VoteExtBodyByHeight(
		sdk.Context{}.WithBlockHeight(10),
		&QueryVoteExtBodyByHeightRequest{VoteExtensionHeight: 8},
	)

	require.Nil(t, response)
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestVoteExtBodyByHeightReturnsInternalForStakingReadFailure(t *testing.T) {
	handler := &ABCIHandler{stakingKeeper: processProposalFailingStakingKeeper{}}

	response, err := handler.VoteExtBodyByHeight(
		sdk.Context{}.WithBlockHeight(10),
		&QueryVoteExtBodyByHeightRequest{VoteExtensionHeight: 8},
	)

	require.Nil(t, response)
	require.Equal(t, codes.Internal, status.Code(err))
}

func newVoteExtBodyQueryTestHandler(t *testing.T, validators []stakingtypes.Validator, cosmosToMina map[string][]byte) *ABCIHandler {
	t.Helper()

	return &ABCIHandler{
		stakingKeeper: quorumTestStakingKeeper{
			validators:           validators,
			validatorsByConsAddr: validatorsByConsAddr(t, validators...),
		},
		keyregistryKeeper: quorumTestKeyregistryKeeper{cosmosToMina: cosmosToMina},
		bridgeKeeper:      testBridgeKeeper{},
	}
}
