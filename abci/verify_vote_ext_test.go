package abci

import (
	"context"
	"fmt"
	"testing"

	cometabci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestVerifyVoteExtensionAcceptsEmptyExtensionWhenDisabled(t *testing.T) {
	handler := &ABCIHandler{}
	ctx := voteExtensionPolicyTestContext(0)

	response, err := handler.VerifyVoteExtensionHandler()(ctx, &cometabci.RequestVerifyVoteExtension{
		Height:        2,
		VoteExtension: nil,
	})

	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseVerifyVoteExtension_ACCEPT, response.Status)
}

func TestVerifyVoteExtensionRejectsNonEmptyExtensionWhenDisabledWithoutError(t *testing.T) {
	handler := &ABCIHandler{}
	ctx := voteExtensionPolicyTestContext(0)

	response, err := handler.VerifyVoteExtensionHandler()(ctx, &cometabci.RequestVerifyVoteExtension{
		Height:        2,
		VoteExtension: []byte("unexpected-extension"),
	})

	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseVerifyVoteExtension_REJECT, response.Status)
}

func TestVerifyVoteExtensionReturnsErrorWhenConsensusParamsUnavailable(t *testing.T) {
	handler := &ABCIHandler{}
	ctx := sdk.Context{}.WithConsensusParams(tmproto.ConsensusParams{})

	response, err := handler.VerifyVoteExtensionHandler()(ctx, &cometabci.RequestVerifyVoteExtension{
		Height: 2,
	})

	require.ErrorIs(t, err, ErrUnableToReadConsensusParams)
	require.Equal(t, cometabci.ResponseVerifyVoteExtension_REJECT, response.Status)
}

func TestVerifyVoteExtensionRejectsUnregisteredValidatorWithoutError(t *testing.T) {
	reqHeight := int64(10)
	validator := newTestBondedValidator(t, 10)
	handler := newVerifyVoteExtensionTestHandler(t, []stakingtypes.Validator{validator}, nil)

	response, err := handler.VerifyVoteExtensionHandler()(prepareProposalTestContext(reqHeight), &cometabci.RequestVerifyVoteExtension{
		Height:           reqHeight,
		ValidatorAddress: consensusAddress(t, validator),
		VoteExtension:    []byte("vote-extension"),
	})

	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseVerifyVoteExtension_REJECT, response.Status)
}

func TestVerifyVoteExtensionReturnsErrorForKeyregistryReadFailure(t *testing.T) {
	handler, ctx, req := newValidVerifyVoteExtensionTestCase(t)
	handler.keyregistryKeeper = verifyVoteExtensionTestKeyregistryKeeper{
		cosmosToMina: map[string][]byte{
			string(consensusPubKeyBytes(t, req.validator)): req.minaPublicKey,
		},
		failGet: true,
	}

	response, err := handler.VerifyVoteExtensionHandler()(ctx, req.request)

	require.Error(t, err)
	require.Equal(t, cometabci.ResponseVerifyVoteExtension_REJECT, response.Status)
}

func TestVerifyVoteExtensionRejectsMalformedSignatureWithoutError(t *testing.T) {
	handler, ctx, req := newValidVerifyVoteExtensionTestCase(t)
	req.request.VoteExtension = []byte("not-a-signature")

	response, err := handler.VerifyVoteExtensionHandler()(ctx, req.request)

	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseVerifyVoteExtension_REJECT, response.Status)
}

func TestVerifyVoteExtensionRejectsSignatureMismatchWithoutError(t *testing.T) {
	handler, ctx, req := newValidVerifyVoteExtensionTestCase(t)
	signature, err := req.secondaryKey.SignVoteExtBody(validVoteExtBody())
	require.NoError(t, err)
	req.request.VoteExtension = signature

	response, err := handler.VerifyVoteExtensionHandler()(ctx, req.request)

	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseVerifyVoteExtension_REJECT, response.Status)
}

func TestVerifyVoteExtensionReturnsErrorForMalformedStoredMinaPublicKey(t *testing.T) {
	handler, ctx, req := newValidVerifyVoteExtensionTestCase(t)
	handler.keyregistryKeeper = verifyVoteExtensionTestKeyregistryKeeper{
		cosmosToMina: map[string][]byte{
			string(consensusPubKeyBytes(t, req.validator)): []byte("malformed-mina-public-key"),
		},
	}

	response, err := handler.VerifyVoteExtensionHandler()(ctx, req.request)

	require.Error(t, err)
	require.Equal(t, cometabci.ResponseVerifyVoteExtension_REJECT, response.Status)
}

func TestVerifyVoteExtensionAcceptsValidSignature(t *testing.T) {
	handler, ctx, req := newValidVerifyVoteExtensionTestCase(t)

	response, err := handler.VerifyVoteExtensionHandler()(ctx, req.request)

	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseVerifyVoteExtension_ACCEPT, response.Status)
}

type verifyVoteExtensionTestCase struct {
	request       *cometabci.RequestVerifyVoteExtension
	validator     stakingtypes.Validator
	secondaryKey  SecondaryKey
	minaPublicKey []byte
}

func newValidVerifyVoteExtensionTestCase(t *testing.T) (*ABCIHandler, sdk.Context, verifyVoteExtensionTestCase) {
	t.Helper()

	reqHeight := int64(10)
	validator := newTestBondedValidator(t, 10)
	secondaryKey := validSecondaryKey()
	minaPublicKey := testMinaPublicKeyFromSecondaryKey(t, secondaryKey)
	handler := newVerifyVoteExtensionTestHandler(t, []stakingtypes.Validator{validator}, map[string][]byte{
		string(consensusPubKeyBytes(t, validator)): minaPublicKey,
	})
	ctx := prepareProposalTestContext(reqHeight)
	body, err := handler.constructVoteExtBody(ctx, reqHeight)
	require.NoError(t, err)
	signature, err := secondaryKey.SignVoteExtBody(body)
	require.NoError(t, err)

	return handler, ctx, verifyVoteExtensionTestCase{
		request: &cometabci.RequestVerifyVoteExtension{
			Height:           reqHeight,
			ValidatorAddress: consensusAddress(t, validator),
			VoteExtension:    signature,
		},
		validator:     validator,
		secondaryKey:  secondaryKey,
		minaPublicKey: minaPublicKey,
	}
}

func newVerifyVoteExtensionTestHandler(t *testing.T, validators []stakingtypes.Validator, cosmosToMina map[string][]byte) *ABCIHandler {
	t.Helper()

	return &ABCIHandler{
		stakingKeeper: quorumTestStakingKeeper{
			validators:           validators,
			validatorsByConsAddr: validatorsByConsAddr(t, validators...),
		},
		keyregistryKeeper: verifyVoteExtensionTestKeyregistryKeeper{cosmosToMina: cosmosToMina},
	}
}

func consensusAddress(t *testing.T, validator stakingtypes.Validator) []byte {
	t.Helper()

	consAddr, err := validator.GetConsAddr()
	require.NoError(t, err)

	return consAddr
}

type verifyVoteExtensionTestKeyregistryKeeper struct {
	cosmosToMina map[string][]byte
	failHas      bool
	failGet      bool
}

func (k verifyVoteExtensionTestKeyregistryKeeper) ValidatorCosmosToMinaHas(_ context.Context, cosmosPubKey []byte) (bool, error) {
	if k.failHas {
		return false, fmt.Errorf("keyregistry has failed")
	}

	_, ok := k.cosmosToMina[string(cosmosPubKey)]
	return ok, nil
}

func (k verifyVoteExtensionTestKeyregistryKeeper) ValidatorGetCosmosToMina(_ context.Context, cosmosPubKey []byte) ([]byte, error) {
	if k.failGet {
		return nil, fmt.Errorf("keyregistry get failed")
	}

	return k.cosmosToMina[string(cosmosPubKey)], nil
}
