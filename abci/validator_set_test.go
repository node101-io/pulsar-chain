package abci

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/stretchr/testify/require"
)

func TestSortValidatorsByPower(t *testing.T) {
	lowPower := newTestBondedValidator(t, 1)
	midPower := newTestBondedValidator(t, 5)
	highPower := newTestBondedValidator(t, 10)
	validators := []stakingtypes.ValidatorI{lowPower, highPower, midPower}

	require.NoError(t, sortValidatorsByPower(validators))

	require.Equal(t, int64(10), validators[0].GetConsensusPower(sdk.DefaultPowerReduction))
	require.Equal(t, int64(5), validators[1].GetConsensusPower(sdk.DefaultPowerReduction))
	require.Equal(t, int64(1), validators[2].GetConsensusPower(sdk.DefaultPowerReduction))
}

func TestSortValidatorsByPowerTiesByConsensusAddress(t *testing.T) {
	validators := []stakingtypes.ValidatorI{
		newTestBondedValidator(t, 5),
		newTestBondedValidator(t, 5),
		newTestBondedValidator(t, 5),
	}
	expectedAddresses := consensusAddresses(t, validators)
	sort.Slice(expectedAddresses, func(i, j int) bool {
		return bytes.Compare(expectedAddresses[i], expectedAddresses[j]) < 0
	})

	require.NoError(t, sortValidatorsByPower(validators))

	require.Equal(t, expectedAddresses, consensusAddresses(t, validators))
}

func TestSortValidatorsByPowerReturnsConsensusAddressError(t *testing.T) {
	validators := []stakingtypes.ValidatorI{failingConsensusAddressValidator{}}

	err := sortValidatorsByPower(validators)

	require.Error(t, err)
}

func TestCalculateValidatorSetRootReturnsMissingMinaKeyError(t *testing.T) {
	validator := newTestBondedValidator(t, 5)
	handler := &ABCIHandler{
		stakingKeeper:     validatorSetTestStakingKeeper{validatorsByConsAddr: validatorsByConsAddr(t, validator)},
		keyregistryKeeper: validatorSetTestKeyregistryKeeper{},
	}

	_, err := handler.calculateValidatorSetRoot(sdk.Context{}, []stakingtypes.ValidatorI{validator}, testPoseidonHash())

	require.ErrorIs(t, err, ErrValidatorMinaKeyNotFound)
}

func TestCalculateValidatorSetRootReturnsHashFailureForNilPoseidon(t *testing.T) {
	handler := &ABCIHandler{}

	root, err := handler.calculateValidatorSetRoot(sdk.Context{}, nil, nil)

	require.Nil(t, root)
	require.ErrorIs(t, err, ErrValidatorSetRootHashFailed)
}

func TestCalculateValidatorSetRootSuccess(t *testing.T) {
	firstValidator := newTestBondedValidator(t, 5)
	secondValidator := newTestBondedValidator(t, 10)
	cosmosToMina := map[string][]byte{
		string(consensusPubKeyBytes(t, firstValidator)):  testMinaPublicKey(t, [32]byte{1}),
		string(consensusPubKeyBytes(t, secondValidator)): testMinaPublicKey(t, [32]byte{2}),
	}
	handler := &ABCIHandler{
		stakingKeeper: validatorSetTestStakingKeeper{
			validatorsByConsAddr: validatorsByConsAddr(t, firstValidator, secondValidator),
		},
		keyregistryKeeper: validatorSetTestKeyregistryKeeper{cosmosToMina: cosmosToMina},
	}

	root, err := handler.calculateValidatorSetRoot(
		sdk.Context{},
		[]stakingtypes.ValidatorI{firstValidator, secondValidator},
		testPoseidonHash(),
	)

	require.NoError(t, err)
	require.NotNil(t, root)
	require.NotEmpty(t, root)
}

func TestConstructVoteExtBodyUsesVoteExtensionHeightSemantics(t *testing.T) {
	voteExtensionHeight := int64(5)
	signedStateHeight := int64(3)
	validator := newTestBondedValidator(t, 10)
	cosmosToMina := map[string][]byte{
		string(consensusPubKeyBytes(t, validator)): testMinaPublicKey(t, [32]byte{1}),
	}
	stakingKeeper := &voteExtBodyTestStakingKeeper{
		validators:           []stakingtypes.Validator{validator},
		validatorsByConsAddr: validatorsByConsAddr(t, validator),
		historicalInfo: map[int64]stakingtypes.HistoricalInfo{
			4: {Header: tmproto.Header{AppHash: []byte("state-after-3")}},
		},
	}
	handler := &ABCIHandler{
		stakingKeeper:     stakingKeeper,
		keyregistryKeeper: validatorSetTestKeyregistryKeeper{cosmosToMina: cosmosToMina},
	}
	ctx := sdk.Context{}.WithBlockHeight(voteExtensionHeight)

	body, err := handler.constructVoteExtBody(ctx, voteExtensionHeight)

	require.NoError(t, err)
	require.Equal(t, []byte("state-after-3"), body.CurrentStateRoot)
	require.Equal(t, signedStateHeight, body.CurrentBlockHeight)
	require.NotEmpty(t, body.NextValidatorSetHash)
	require.Equal(t, []int64{4}, stakingKeeper.requestedHistoricalHeights)
}

func newTestBondedValidator(t *testing.T, power int64) stakingtypes.Validator {
	t.Helper()

	consPubKey := ed25519.GenPrivKey().PubKey()
	validator, err := stakingtypes.NewValidator(
		sdk.ValAddress(consPubKey.Address()).String(),
		consPubKey,
		stakingtypes.Description{},
	)
	require.NoError(t, err)

	validator = validator.UpdateStatus(stakingtypes.Bonded)
	validator.Tokens = sdk.TokensFromConsensusPower(power, sdk.DefaultPowerReduction)

	return validator
}

func consensusAddresses(t *testing.T, validators []stakingtypes.ValidatorI) [][]byte {
	t.Helper()

	addresses := make([][]byte, 0, len(validators))
	for _, validator := range validators {
		consAddr, err := validator.GetConsAddr()
		require.NoError(t, err)
		addresses = append(addresses, consAddr)
	}

	return addresses
}

func validatorsByConsAddr(t *testing.T, validators ...stakingtypes.Validator) map[string]stakingtypes.Validator {
	t.Helper()

	byConsAddr := make(map[string]stakingtypes.Validator, len(validators))
	for _, validator := range validators {
		consAddr, err := validator.GetConsAddr()
		require.NoError(t, err)
		byConsAddr[string(consAddr)] = validator
	}

	return byConsAddr
}

func consensusPubKeyBytes(t *testing.T, validator stakingtypes.Validator) []byte {
	t.Helper()

	consPubKey, err := validator.ConsPubKey()
	require.NoError(t, err)

	return consPubKey.Bytes()
}

func testMinaPublicKey(t *testing.T, seed [32]byte) []byte {
	t.Helper()

	privateKey, err := privatekey.NewPrivateKeyFromBytes(seed, NetworkID)
	require.NoError(t, err)
	publicKey, err := privateKey.ToPublicKey()
	require.NoError(t, err)

	return publicKey.Bytes()
}

type validatorSetTestStakingKeeper struct {
	validatorsByConsAddr map[string]stakingtypes.Validator
}

func (validatorSetTestStakingKeeper) IterateLastValidators(context.Context, func(int64, stakingtypes.ValidatorI) bool) error {
	return nil
}

func (validatorSetTestStakingKeeper) GetHistoricalInfo(context.Context, int64) (stakingtypes.HistoricalInfo, error) {
	return stakingtypes.HistoricalInfo{}, nil
}

func (k validatorSetTestStakingKeeper) GetValidatorByConsAddr(_ context.Context, consAddr sdk.ConsAddress) (stakingtypes.Validator, error) {
	validator, ok := k.validatorsByConsAddr[string(consAddr)]
	if !ok {
		return stakingtypes.Validator{}, fmt.Errorf("validator not found")
	}

	return validator, nil
}

type validatorSetTestKeyregistryKeeper struct {
	cosmosToMina map[string][]byte
}

func (k validatorSetTestKeyregistryKeeper) ValidatorCosmosToMinaHas(_ context.Context, cosmosPubKey []byte) (bool, error) {
	_, ok := k.cosmosToMina[string(cosmosPubKey)]
	return ok, nil
}

func (k validatorSetTestKeyregistryKeeper) ValidatorGetCosmosToMina(_ context.Context, cosmosPubKey []byte) ([]byte, error) {
	minaPubKey, ok := k.cosmosToMina[string(cosmosPubKey)]
	if !ok {
		return nil, fmt.Errorf("mina public key not found")
	}

	return minaPubKey, nil
}

type failingConsensusAddressValidator struct {
	stakingtypes.Validator
}

func (failingConsensusAddressValidator) GetConsAddr() ([]byte, error) {
	return nil, fmt.Errorf("consensus address failed")
}

type voteExtBodyTestStakingKeeper struct {
	validators                 []stakingtypes.Validator
	validatorsByConsAddr       map[string]stakingtypes.Validator
	historicalInfo             map[int64]stakingtypes.HistoricalInfo
	requestedHistoricalHeights []int64
}

func (k *voteExtBodyTestStakingKeeper) IterateLastValidators(_ context.Context, fn func(int64, stakingtypes.ValidatorI) bool) error {
	for i, validator := range k.validators {
		if fn(int64(i), validator) {
			break
		}
	}

	return nil
}

func (k *voteExtBodyTestStakingKeeper) GetHistoricalInfo(_ context.Context, height int64) (stakingtypes.HistoricalInfo, error) {
	k.requestedHistoricalHeights = append(k.requestedHistoricalHeights, height)
	historicalInfo, ok := k.historicalInfo[height]
	if !ok {
		return stakingtypes.HistoricalInfo{}, fmt.Errorf("historical info not found")
	}

	return historicalInfo, nil
}

func (k *voteExtBodyTestStakingKeeper) GetValidatorByConsAddr(_ context.Context, consAddr sdk.ConsAddress) (stakingtypes.Validator, error) {
	validator, ok := k.validatorsByConsAddr[string(consAddr)]
	if !ok {
		return stakingtypes.Validator{}, fmt.Errorf("validator not found")
	}

	return validator, nil
}
