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

func TestGetValidatorSetWithPowersRejectsNilRequest(t *testing.T) {
	handler := &ABCIHandler{}

	response, err := handler.GetValidatorSetWithPowers(context.Background(), nil)

	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetValidatorSetWithPowersRejectsNonPositiveHeight(t *testing.T) {
	handler := &ABCIHandler{}

	response, err := handler.GetValidatorSetWithPowers(
		sdk.Context{}.WithBlockHeight(10),
		&QueryGetValidatorSetWithPowersRequest{BlockHeight: 0},
	)

	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetValidatorSetWithPowersRejectsFutureHeight(t *testing.T) {
	handler := &ABCIHandler{}

	response, err := handler.GetValidatorSetWithPowers(
		sdk.Context{}.WithBlockHeight(10),
		&QueryGetValidatorSetWithPowersRequest{BlockHeight: 11},
	)

	require.Nil(t, response)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetValidatorSetWithPowersReturnsCurrentValidatorSetOrderedByPower(t *testing.T) {
	lowPowerValidator := newTestBondedValidator(t, 1)
	highPowerValidator := newTestBondedValidator(t, 10)
	midPowerValidator := newTestBondedValidator(t, 5)
	handler := &ABCIHandler{
		stakingKeeper: quorumTestStakingKeeper{
			validators: []stakingtypes.Validator{
				lowPowerValidator,
				highPowerValidator,
				midPowerValidator,
			},
		},
	}

	response, err := handler.GetValidatorSetWithPowers(
		sdk.Context{}.WithBlockHeight(10),
		&QueryGetValidatorSetWithPowersRequest{BlockHeight: 10},
	)

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Len(t, response.GetValidators(), 3)
	require.Equal(t, consensusPubKeyBytes(t, highPowerValidator), response.GetValidators()[0].GetValidatorCosmosPubKey())
	require.Equal(t, int64(10), response.GetValidators()[0].GetConsensusPower())
	require.Equal(t, consensusPubKeyBytes(t, midPowerValidator), response.GetValidators()[1].GetValidatorCosmosPubKey())
	require.Equal(t, int64(5), response.GetValidators()[1].GetConsensusPower())
	require.Equal(t, consensusPubKeyBytes(t, lowPowerValidator), response.GetValidators()[2].GetValidatorCosmosPubKey())
	require.Equal(t, int64(1), response.GetValidators()[2].GetConsensusPower())
}

func TestGetValidatorSetWithPowersReturnsHistoricalValidatorSet(t *testing.T) {
	currentValidator := newTestBondedValidator(t, 20)
	firstHistoricalValidator := newTestBondedValidator(t, 10)
	secondHistoricalValidator := newTestBondedValidator(t, 5)
	stakingKeeper := &voteExtBodyTestStakingKeeper{
		validators: []stakingtypes.Validator{currentValidator},
		historicalInfo: map[int64]stakingtypes.HistoricalInfo{
			7: {
				Valset: []stakingtypes.Validator{
					firstHistoricalValidator,
					secondHistoricalValidator,
				},
			},
		},
	}
	handler := &ABCIHandler{stakingKeeper: stakingKeeper}

	response, err := handler.GetValidatorSetWithPowers(
		sdk.Context{}.WithBlockHeight(10),
		&QueryGetValidatorSetWithPowersRequest{BlockHeight: 7},
	)

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, []int64{7}, stakingKeeper.requestedHistoricalHeights)
	require.Len(t, response.GetValidators(), 2)
	require.Equal(t, consensusPubKeyBytes(t, firstHistoricalValidator), response.GetValidators()[0].GetValidatorCosmosPubKey())
	require.Equal(t, int64(10), response.GetValidators()[0].GetConsensusPower())
	require.Equal(t, consensusPubKeyBytes(t, secondHistoricalValidator), response.GetValidators()[1].GetValidatorCosmosPubKey())
	require.Equal(t, int64(5), response.GetValidators()[1].GetConsensusPower())
}

func TestGetValidatorSetUsesHeight(t *testing.T) {
	requestedHistoricalHeight := int64(10)
	currentHeight := requestedHistoricalHeight + 1

	// Height 10 has the original validator set A, B, C.
	height10LowPowerValidator := newTestBondedValidator(t, 1)
	height10HighPowerValidator := newTestBondedValidator(t, 10)
	height10MidPowerValidator := newTestBondedValidator(t, 5)
	height10CurrentIterationOrder := []stakingtypes.Validator{
		height10LowPowerValidator,
		height10HighPowerValidator,
		height10MidPowerValidator,
	}
	height10HistoricalValidatorSet := []stakingtypes.Validator{
		height10HighPowerValidator,
		height10MidPowerValidator,
		height10LowPowerValidator,
	}

	// Height 11 keeps A, B, C and adds one new validator D.
	height11NewValidator := newTestBondedValidator(t, 11)
	height11CurrentIterationOrder := []stakingtypes.Validator{
		height10MidPowerValidator,
		height10LowPowerValidator,
		height11NewValidator,
		height10HighPowerValidator,
	}

	height10Handler := &ABCIHandler{
		stakingKeeper: &voteExtBodyTestStakingKeeper{
			validators: height10CurrentIterationOrder,
		},
	}

	// Querying height 10 while the chain is still at height 10 must read the
	// current validator set from IterateLastValidators.
	height10CurrentResponse, err := height10Handler.GetValidatorSetWithPowers(
		sdk.Context{}.WithBlockHeight(requestedHistoricalHeight),
		&QueryGetValidatorSetWithPowersRequest{BlockHeight: requestedHistoricalHeight},
	)
	require.NoError(t, err)
	require.NotNil(t, height10CurrentResponse)

	height11StakingKeeper := &voteExtBodyTestStakingKeeper{
		validators: height11CurrentIterationOrder,
		historicalInfo: map[int64]stakingtypes.HistoricalInfo{
			requestedHistoricalHeight: {
				Valset: height10HistoricalValidatorSet,
			},
		},
	}
	height11Handler := &ABCIHandler{stakingKeeper: height11StakingKeeper}

	// After the chain advances to height 11, querying height 10 must still read
	// the old set from HistoricalInfo(10), not the new current set with D.
	height10HistoricalResponse, err := height11Handler.GetValidatorSetWithPowers(
		sdk.Context{}.WithBlockHeight(currentHeight),
		&QueryGetValidatorSetWithPowersRequest{BlockHeight: requestedHistoricalHeight},
	)
	require.NoError(t, err)
	require.NotNil(t, height10HistoricalResponse)

	// Querying height 11 at height 11 must read the new current set that now
	// includes D.
	height11CurrentResponse, err := height11Handler.GetValidatorSetWithPowers(
		sdk.Context{}.WithBlockHeight(currentHeight),
		&QueryGetValidatorSetWithPowersRequest{BlockHeight: currentHeight},
	)
	require.NoError(t, err)
	require.NotNil(t, height11CurrentResponse)

	require.Equal(t, []int64{requestedHistoricalHeight}, height11StakingKeeper.requestedHistoricalHeights)

	require.Equal(t, height10CurrentResponse.GetValidators(), height10HistoricalResponse.GetValidators())
	require.Equal(t, consensusPubKeyBytes(t, height10HighPowerValidator), height10HistoricalResponse.GetValidators()[0].GetValidatorCosmosPubKey())
	require.Equal(t, consensusPubKeyBytes(t, height10MidPowerValidator), height10HistoricalResponse.GetValidators()[1].GetValidatorCosmosPubKey())
	require.Equal(t, consensusPubKeyBytes(t, height10LowPowerValidator), height10HistoricalResponse.GetValidators()[2].GetValidatorCosmosPubKey())

	require.NotEqual(t, height10HistoricalResponse.GetValidators(), height11CurrentResponse.GetValidators())
	require.Len(t, height11CurrentResponse.GetValidators(), 4)
	require.Equal(t, consensusPubKeyBytes(t, height11NewValidator), height11CurrentResponse.GetValidators()[0].GetValidatorCosmosPubKey())
	require.Equal(t, consensusPubKeyBytes(t, height10HighPowerValidator), height11CurrentResponse.GetValidators()[1].GetValidatorCosmosPubKey())
	require.Equal(t, consensusPubKeyBytes(t, height10MidPowerValidator), height11CurrentResponse.GetValidators()[2].GetValidatorCosmosPubKey())
	require.Equal(t, consensusPubKeyBytes(t, height10LowPowerValidator), height11CurrentResponse.GetValidators()[3].GetValidatorCosmosPubKey())
}

func TestGetValidatorSetWithPowersReturnsIterateLastValidatorsError(t *testing.T) {
	handler := &ABCIHandler{stakingKeeper: processProposalFailingStakingKeeper{}}

	response, err := handler.GetValidatorSetWithPowers(
		sdk.Context{}.WithBlockHeight(10),
		&QueryGetValidatorSetWithPowersRequest{BlockHeight: 10},
	)

	require.Nil(t, response)
	require.ErrorContains(t, err, "iterate validators failed")
}

func TestGetValidatorSetWithPowersReturnsHistoricalInfoError(t *testing.T) {
	handler := &ABCIHandler{stakingKeeper: processProposalFailingStakingKeeper{}}

	response, err := handler.GetValidatorSetWithPowers(
		sdk.Context{}.WithBlockHeight(10),
		&QueryGetValidatorSetWithPowersRequest{BlockHeight: 8},
	)

	require.Nil(t, response)
	require.ErrorContains(t, err, "historical info failed")
}
