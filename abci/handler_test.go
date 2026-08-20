package abci

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	verificationvalidator "github.com/node101-io/pulsar-chain/x/verification/validator"
)

func TestNewABCIHandlerValidatesSecondaryKey(t *testing.T) {
	handler, err := NewABCIHandler(SecondaryKey{}, testStakingKeeper{}, testKeyregistryKeeper{}, testVotePersistenceKeeper{}, NetworkID, testBridgeKeeper{}, nil, nil)

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
		expectedErr           error
	}{
		{
			name:                  "missing staking keeper",
			stakingKeeper:         nil,
			keyregistryKeeper:     testKeyregistryKeeper{},
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			expectedErr:           ErrMissingStakingKeeper,
		},
		{
			name:                  "missing keyregistry keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     nil,
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			expectedErr:           ErrMissingKeyregistryKeeper,
		},
		{
			name:                  "missing vote persistence keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     testKeyregistryKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			votePersistenceKeeper: nil,
			expectedErr:           ErrMissingVotePersistenceKeeper,
		},
		{
			name:                  "missing bridge keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     testKeyregistryKeeper{},
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          nil,
			expectedErr:           ErrMissingBridgeKeeper,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewABCIHandler(secondaryKey, tt.stakingKeeper, tt.keyregistryKeeper, tt.votePersistenceKeeper, NetworkID, tt.bridgeKeeper, nil, nil)

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

	tests := []struct {
		name                  string
		stakingKeeper         StakingKeeper
		keyregistryKeeper     KeyregistryKeeper
		votePersistenceKeeper VotePersistenceKeeper
		bridgeKeeper          BridgeKeeper
		expectedErr           error
	}{
		{
			name:                  "typed nil staking keeper",
			stakingKeeper:         typedNilStakingKeeper,
			keyregistryKeeper:     testKeyregistryKeeper{},
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			expectedErr:           ErrMissingStakingKeeper,
		},
		{
			name:                  "typed nil keyregistry keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     typedNilKeyregistryKeeper,
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			expectedErr:           ErrMissingKeyregistryKeeper,
		},
		{
			name:                  "typed nil vote persistence keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     testKeyregistryKeeper{},
			bridgeKeeper:          testBridgeKeeper{},
			votePersistenceKeeper: typedNilVotePersistenceKeeper,
			expectedErr:           ErrMissingVotePersistenceKeeper,
		},
		{
			name:                  "typed nil bridge keeper",
			stakingKeeper:         testStakingKeeper{},
			keyregistryKeeper:     testKeyregistryKeeper{},
			votePersistenceKeeper: testVotePersistenceKeeper{},
			bridgeKeeper:          typedNilBridgeKeeper,
			expectedErr:           ErrMissingBridgeKeeper,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewABCIHandler(secondaryKey, tt.stakingKeeper, tt.keyregistryKeeper, tt.votePersistenceKeeper, NetworkID, tt.bridgeKeeper, nil, nil)
			require.Nil(t, handler)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestNewABCIHandlerRequiresVerificationDependencies(t *testing.T) {
	keeper := &verificationKeeperStub{}
	builder := verificationBuilderStub{}

	tests := []struct {
		name    string
		keeper  VerificationKeeper
		builder VerificationPayloadBuilder
		err     error
	}{
		{name: "missing verification keeper", builder: builder, err: ErrMissingVerificationKeeper},
		{name: "missing verification builder", keeper: keeper, err: ErrMissingVerificationBuilder},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			handler, err := NewABCIHandler(
				validSecondaryKey(),
				testStakingKeeper{},
				testKeyregistryKeeper{},
				testVotePersistenceKeeper{},
				NetworkID,
				testBridgeKeeper{},
				testCase.keeper,
				testCase.builder,
			)
			require.Nil(t, handler)
			require.ErrorIs(t, err, testCase.err)
		})
	}
}

func TestNewABCIHandlerRejectsTypedNilVerificationDependencies(t *testing.T) {
	var keeper *verificationKeeperStub
	var builder *verificationBuilderStub

	for _, testCase := range []struct {
		name    string
		keeper  VerificationKeeper
		builder VerificationPayloadBuilder
		err     error
	}{
		{name: "typed nil verification keeper", keeper: keeper, builder: verificationBuilderStub{}, err: ErrMissingVerificationKeeper},
		{name: "typed nil verification builder", keeper: &verificationKeeperStub{}, builder: builder, err: ErrMissingVerificationBuilder},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			handler, err := NewABCIHandler(
				validSecondaryKey(),
				testStakingKeeper{},
				testKeyregistryKeeper{},
				testVotePersistenceKeeper{},
				NetworkID,
				testBridgeKeeper{},
				testCase.keeper,
				testCase.builder,
			)
			require.Nil(t, handler)
			require.ErrorIs(t, err, testCase.err)
		})
	}
}

func TestNewABCIHandlerSuccess(t *testing.T) {
	handler, err := NewABCIHandler(
		validSecondaryKey(),
		testStakingKeeper{},
		testKeyregistryKeeper{},
		testVotePersistenceKeeper{},
		NetworkID,
		testBridgeKeeper{},
		&verificationKeeperStub{},
		verificationvalidator.DisabledBuilder{},
	)

	require.NoError(t, err)
	require.NotNil(t, handler)
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
