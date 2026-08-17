package abci

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestNewABCIHandlerValidatesSecondaryKey(t *testing.T) {
	handler, err := NewABCIHandler(
		SecondaryKey{},
		testStakingKeeper{},
		testKeyregistryKeeper{},
		testVotePersistenceKeeper{},
		NetworkID,
		testBridgeKeeper{},
		testVerificationKeeper{},
	)

	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrMissingSecondaryKey)
}

func TestNewABCIHandlerValidatesKeeperDependencies(t *testing.T) {
	secondaryKey := validSecondaryKey()

	tests := []struct {
		name                  string
		stakingKeeper         StakingKeeper
		keyregistryKeeper     KeyregistryKeeper
		votePersistenceKeeper VotePersistenceKeeper
		bridgeKeeper          BridgeKeeper
		verificationKeeper    VerificationKeeper
		expectedErr           error
	}{
		{
			name:                  "missing staking keeper",
			stakingKeeper:         nil,
			keyregistryKeeper:     testKeyregistryKeeper{},
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			verificationKeeper:    testVerificationKeeper{},
			expectedErr:           ErrMissingStakingKeeper,
		},
		{
			name:                  "missing keyregistry keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     nil,
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			verificationKeeper:    testVerificationKeeper{},
			expectedErr:           ErrMissingKeyregistryKeeper,
		},
		{
			name:                  "missing vote persistence keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     testKeyregistryKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			votePersistenceKeeper: nil,
			verificationKeeper:    testVerificationKeeper{},
			expectedErr:           ErrMissingVotePersistenceKeeper,
		},
		{
			name:                  "missing bridge keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     testKeyregistryKeeper{},
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          nil,
			verificationKeeper:    testVerificationKeeper{},
			expectedErr:           ErrMissingBridgeKeeper,
		},
		{
			name:                  "missing verification keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     testKeyregistryKeeper{},
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			verificationKeeper:    nil,
			expectedErr:           ErrMissingVerificationKeeper,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewABCIHandler(
				secondaryKey,
				tt.stakingKeeper,
				tt.keyregistryKeeper,
				tt.votePersistenceKeeper,
				NetworkID,
				tt.bridgeKeeper,
				tt.verificationKeeper,
			)

			require.Nil(t, handler)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestNewABCIHandlerRejectsTypedNilKeeperDependencies(t *testing.T) {
	secondaryKey := validSecondaryKey()
	var typedNilStakingKeeper *testStakingKeeper
	var typedNilKeyregistryKeeper *testKeyregistryKeeper
	var typedNilVotePersistenceKeeper *testVotePersistenceKeeper
	var typedNilBridgeKeeper *testBridgeKeeper
	var typedNilVerificationKeeper *testVerificationKeeper

	tests := []struct {
		name                  string
		stakingKeeper         StakingKeeper
		keyregistryKeeper     KeyregistryKeeper
		votePersistenceKeeper VotePersistenceKeeper
		bridgeKeeper          BridgeKeeper
		verificationKeeper    VerificationKeeper
		expectedErr           error
	}{
		{
			name:                  "typed nil staking keeper",
			stakingKeeper:         typedNilStakingKeeper,
			keyregistryKeeper:     testKeyregistryKeeper{},
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			verificationKeeper:    testVerificationKeeper{},
			expectedErr:           ErrMissingStakingKeeper,
		},
		{
			name:                  "typed nil keyregistry keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     typedNilKeyregistryKeeper,
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			verificationKeeper:    testVerificationKeeper{},
			expectedErr:           ErrMissingKeyregistryKeeper,
		},
		{
			name:                  "typed nil vote persistence keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     testKeyregistryKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			votePersistenceKeeper: typedNilVotePersistenceKeeper,
			verificationKeeper:    testVerificationKeeper{},
			expectedErr:           ErrMissingVotePersistenceKeeper,
		},
		{
			name:                  "typed nil bridge keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     testKeyregistryKeeper{},
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          typedNilBridgeKeeper,
			verificationKeeper:    testVerificationKeeper{},
			expectedErr:           ErrMissingBridgeKeeper,
		},
		{
			name:                  "typed nil verification keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     testKeyregistryKeeper{},
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			verificationKeeper:    typedNilVerificationKeeper,
			expectedErr:           ErrMissingVerificationKeeper,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewABCIHandler(
				secondaryKey,
				tt.stakingKeeper,
				tt.keyregistryKeeper,
				tt.votePersistenceKeeper,
				NetworkID,
				tt.bridgeKeeper,
				tt.verificationKeeper,
			)
			require.Nil(t, handler)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestNewABCIHandlerSuccess(t *testing.T) {
	verificationKeeper := testVerificationKeeper{}
	handler, err := NewABCIHandler(
		validSecondaryKey(),
		testStakingKeeper{},
		testKeyregistryKeeper{},
		testVotePersistenceKeeper{},
		NetworkID,
		testBridgeKeeper{},
		verificationKeeper,
	)

	require.NoError(t, err)
	require.NotNil(t, handler)
	require.Equal(t, verificationKeeper, handler.verificationKeeper)
}

type testStakingKeeper struct{}

func (testStakingKeeper) IterateLastValidators(context.Context, func(int64, stakingtypes.ValidatorI) bool) error {
	return nil
}

func (testStakingKeeper) GetHistoricalInfo(context.Context, int64) (stakingtypes.HistoricalInfo, error) {
	return stakingtypes.HistoricalInfo{}, nil
}

func (testStakingKeeper) GetValidatorByConsAddr(context.Context, sdk.ConsAddress) (stakingtypes.Validator, error) {
	return stakingtypes.Validator{}, nil
}

type testKeyregistryKeeper struct{}

func (testKeyregistryKeeper) ValidatorCosmosToMinaHas(context.Context, []byte) (bool, error) {
	return false, nil
}

func (testKeyregistryKeeper) ValidatorGetCosmosToMina(context.Context, []byte) ([]byte, error) {
	return nil, nil
}

type testVotePersistenceKeeper struct{}

func (testVotePersistenceKeeper) Clear(context.Context) error {
	return nil
}

func (testVotePersistenceKeeper) SetVote(context.Context, int64, []byte, []byte) error {
	return nil
}

func (testVotePersistenceKeeper) GetVote(context.Context, int64, []byte) ([]byte, error) {
	return nil, nil
}

type testVerificationKeeper struct{}

func (testVerificationKeeper) GetProofHashesByBlockHeight(
	context.Context,
	int64,
) ([][]byte, []int64, error) {
	return nil, nil, nil
}
