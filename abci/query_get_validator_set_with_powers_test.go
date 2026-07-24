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
			8: {
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
	require.Equal(t, []int64{8}, stakingKeeper.requestedHistoricalHeights)
	require.Len(t, response.GetValidators(), 2)
	require.Equal(t, consensusPubKeyBytes(t, firstHistoricalValidator), response.GetValidators()[0].GetValidatorCosmosPubKey())
	require.Equal(t, int64(10), response.GetValidators()[0].GetConsensusPower())
	require.Equal(t, consensusPubKeyBytes(t, secondHistoricalValidator), response.GetValidators()[1].GetValidatorCosmosPubKey())
	require.Equal(t, int64(5), response.GetValidators()[1].GetConsensusPower())
}

func TestGetValidatorSetUsesHeight(t *testing.T) {
	const requestedHeight int64 = 10
	nextHeight := requestedHeight + 1

	lowPowerValidator := newTestBondedValidator(t, 1)
	highPowerValidator := newTestBondedValidator(t, 10)
	newValidator := newTestBondedValidator(t, 11)

	initialValidatorSet := []stakingtypes.Validator{
		highPowerValidator,
		lowPowerValidator,
	}
	validatorSetAfterTx := []stakingtypes.Validator{
		lowPowerValidator,
		newValidator,
		highPowerValidator,
	}

	stakingKeeper := &voteExtBodyTestStakingKeeper{
		validators: initialValidatorSet,
		historicalInfo: map[int64]stakingtypes.HistoricalInfo{
			// BeginBlock(N) snapshots the validator set that existed before
			// transactions and EndBlock validator updates at height N.
			requestedHeight: {
				Valset: initialValidatorSet,
			},
		},
	}
	handler := &ABCIHandler{stakingKeeper: stakingKeeper}

	// A staking transaction at height N changes the pending validator set.
	// Staking applies that set to LastValidators during EndBlock(N).
	stakingKeeper.validators = validatorSetAfterTx

	currentResponse, err := handler.GetValidatorSetWithPowers(
		sdk.Context{}.WithBlockHeight(requestedHeight),
		&QueryGetValidatorSetWithPowersRequest{BlockHeight: requestedHeight},
	)
	require.NoError(t, err)
	require.NotNil(t, currentResponse)

	// BeginBlock(N+1) snapshots the LastValidators produced by EndBlock(N).
	historicalValidators := append([]stakingtypes.Validator(nil), validatorSetAfterTx...)
	stakingKeeper.historicalInfo[nextHeight] = stakingtypes.NewHistoricalInfo(
		sdk.Context{}.WithBlockHeight(nextHeight).BlockHeader(),
		stakingtypes.Validators{Validators: historicalValidators},
		sdk.DefaultPowerReduction,
	)

	historicalResponse, err := handler.GetValidatorSetWithPowers(
		sdk.Context{}.WithBlockHeight(nextHeight),
		&QueryGetValidatorSetWithPowersRequest{BlockHeight: requestedHeight},
	)
	require.NoError(t, err)
	require.NotNil(t, historicalResponse)

	require.Equal(t, []int64{nextHeight}, stakingKeeper.requestedHistoricalHeights)
	require.Equal(t, currentResponse.GetValidators(), historicalResponse.GetValidators())
	require.Len(t, historicalResponse.GetValidators(), 3)
	require.Equal(t, consensusPubKeyBytes(t, newValidator), historicalResponse.GetValidators()[0].GetValidatorCosmosPubKey())
	require.Equal(t, consensusPubKeyBytes(t, highPowerValidator), historicalResponse.GetValidators()[1].GetValidatorCosmosPubKey())
	require.Equal(t, consensusPubKeyBytes(t, lowPowerValidator), historicalResponse.GetValidators()[2].GetValidatorCosmosPubKey())
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
