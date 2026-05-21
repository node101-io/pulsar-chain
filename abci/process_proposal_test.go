package abci

import (
	"context"
	"fmt"
	"testing"

	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestProcessProposalAcceptsWhenVoteExtensionsDisabled(t *testing.T) {
	handler := &ABCIHandler{}
	ctx := sdk.Context{}.WithConsensusParams(prepareProposalTestContext(10).ConsensusParams())
	req := &cometabci.RequestProcessProposal{Height: 2}

	response, err := handler.ProcessProposalHandler()(ctx, req)

	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseProcessProposal_ACCEPT, response.Status)
}

func TestProcessProposalRejectsMissingPayloadWithoutError(t *testing.T) {
	handler, ctx, reqHeight, _ := newProcessProposalTestCase(t)

	response, err := handler.ProcessProposalHandler()(ctx, &cometabci.RequestProcessProposal{
		Height: reqHeight,
		Txs:    [][]byte{[]byte("user-tx")},
	})

	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseProcessProposal_REJECT, response.Status)
}

func TestProcessProposalRejectsMalformedPayloadWithoutError(t *testing.T) {
	handler, ctx, reqHeight, _ := newProcessProposalTestCase(t)
	malformedPayload := append(voteExtMarkerBytes[:len(voteExtMarkerBytes):len(voteExtMarkerBytes)], []byte{0xff, 0xff, 0xff}...)

	response, err := handler.ProcessProposalHandler()(ctx, &cometabci.RequestProcessProposal{
		Height: reqHeight,
		Txs:    [][]byte{malformedPayload},
	})

	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseProcessProposal_REJECT, response.Status)
}

func TestProcessProposalRejectsWrongPayloadHeightWithoutError(t *testing.T) {
	handler, ctx, reqHeight, payload := newProcessProposalTestCase(t)
	payload.VoteExtensionHeight = reqHeight - 2

	response, err := handler.ProcessProposalHandler()(ctx, &cometabci.RequestProcessProposal{
		Height: reqHeight,
		Txs:    [][]byte{markedPayloadTx(t, payload)},
	})

	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseProcessProposal_REJECT, response.Status)
}

func TestProcessProposalRejectsInvalidSignatureWithoutError(t *testing.T) {
	handler, ctx, reqHeight, payload := newProcessProposalTestCase(t)
	payload.VoteExtensions[0].VoteExtension = []byte("invalid-signature")

	response, err := handler.ProcessProposalHandler()(ctx, &cometabci.RequestProcessProposal{
		Height: reqHeight,
		Txs:    [][]byte{markedPayloadTx(t, payload)},
	})

	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseProcessProposal_REJECT, response.Status)
}

func TestProcessProposalRejectsNotEnoughPowerWithoutError(t *testing.T) {
	reqHeight := int64(10)
	firstValidator := newTestBondedValidator(t, 1)
	secondValidator := newTestBondedValidator(t, 2)
	firstSecondaryKey := validSecondaryKey()
	secondSecondaryKey := secondaryKeyFromSeed([32]byte{2})
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{firstValidator, secondValidator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, firstValidator)):  firstSecondaryKey,
		string(consensusPubKeyBytes(t, secondValidator)): secondSecondaryKey,
	}, nil)
	ctx := prepareProposalTestContext(reqHeight)
	body, err := handler.constructVoteExtBody(ctx, reqHeight-1)
	require.NoError(t, err)
	payload := Payload{
		VoteExtensionHeight: reqHeight - 1,
		VoteExtensions: []*PayloadVoteExtension{
			signedPayloadVoteExtension(t, firstValidator, firstSecondaryKey, body),
		},
	}

	response, err := handler.ProcessProposalHandler()(ctx, &cometabci.RequestProcessProposal{
		Height: reqHeight,
		Txs:    [][]byte{markedPayloadTx(t, payload)},
	})

	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseProcessProposal_REJECT, response.Status)
}

func TestProcessProposalReturnsErrorForConstructBodyFailure(t *testing.T) {
	handler := &ABCIHandler{stakingKeeper: processProposalFailingStakingKeeper{}}
	reqHeight := int64(10)

	response, err := handler.ProcessProposalHandler()(prepareProposalTestContext(reqHeight), &cometabci.RequestProcessProposal{
		Height: reqHeight,
	})

	require.Error(t, err)
	require.Equal(t, cometabci.ResponseProcessProposal_REJECT, response.Status)
}

func TestProcessProposalReturnsErrorForKeyregistryReadFailure(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	secondaryKey := validSecondaryKey()
	cosmosPubKey := consensusPubKeyBytes(t, validator)
	minaPubKey := testMinaPublicKeyFromSecondaryKey(t, secondaryKey)
	keyregistryKeeper := &processProposalStatefulKeyregistryKeeper{
		cosmosToMina: map[string][]byte{string(cosmosPubKey): minaPubKey},
		failGetAfter: 1,
	}
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, nil, nil)
	handler.keyregistryKeeper = keyregistryKeeper
	reqHeight := int64(10)
	ctx := prepareProposalTestContext(reqHeight)
	body, err := handler.constructVoteExtBody(ctx, reqHeight-1)
	require.NoError(t, err)
	payload := Payload{
		VoteExtensionHeight: reqHeight - 1,
		VoteExtensions: []*PayloadVoteExtension{
			signedPayloadVoteExtension(t, validator, secondaryKey, body),
		},
	}

	response, err := handler.ProcessProposalHandler()(ctx, &cometabci.RequestProcessProposal{
		Height: reqHeight,
		Txs:    [][]byte{markedPayloadTx(t, payload)},
	})

	require.Error(t, err)
	require.Equal(t, cometabci.ResponseProcessProposal_REJECT, response.Status)
}

func TestProcessProposalReturnsErrorForMalformedStoredMinaPublicKey(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	secondaryKey := validSecondaryKey()
	cosmosPubKey := consensusPubKeyBytes(t, validator)
	minaPubKey := testMinaPublicKeyFromSecondaryKey(t, secondaryKey)
	keyregistryKeeper := &processProposalStatefulKeyregistryKeeper{
		cosmosToMina: map[string][]byte{string(cosmosPubKey): minaPubKey},
		overrideGetAfter: map[int][]byte{
			2: []byte("malformed-mina-key"),
		},
	}
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, nil, nil)
	handler.keyregistryKeeper = keyregistryKeeper
	reqHeight := int64(10)
	ctx := prepareProposalTestContext(reqHeight)
	body, err := handler.constructVoteExtBody(ctx, reqHeight-1)
	require.NoError(t, err)
	payload := Payload{
		VoteExtensionHeight: reqHeight - 1,
		VoteExtensions: []*PayloadVoteExtension{
			signedPayloadVoteExtension(t, validator, secondaryKey, body),
		},
	}

	response, err := handler.ProcessProposalHandler()(ctx, &cometabci.RequestProcessProposal{
		Height: reqHeight,
		Txs:    [][]byte{markedPayloadTx(t, payload)},
	})

	require.ErrorIs(t, err, ErrInvalidVoteExtMinaPublicKey)
	require.Equal(t, cometabci.ResponseProcessProposal_REJECT, response.Status)
}

func newProcessProposalTestCase(t *testing.T) (*ABCIHandler, sdk.Context, int64, Payload) {
	t.Helper()

	reqHeight := int64(10)
	validator := newTestBondedValidator(t, 10)
	secondaryKey := validSecondaryKey()
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, validator)): secondaryKey,
	}, nil)
	ctx := prepareProposalTestContext(reqHeight)
	body, err := handler.constructVoteExtBody(ctx, reqHeight-1)
	require.NoError(t, err)
	payload := Payload{
		VoteExtensionHeight: reqHeight - 1,
		VoteExtensions: []*PayloadVoteExtension{
			signedPayloadVoteExtension(t, validator, secondaryKey, body),
		},
	}

	return handler, ctx, reqHeight, payload
}

type processProposalFailingStakingKeeper struct{}

func (processProposalFailingStakingKeeper) IterateLastValidators(context.Context, func(int64, stakingtypes.ValidatorI) bool) error {
	return fmt.Errorf("iterate validators failed")
}

func (processProposalFailingStakingKeeper) GetHistoricalInfo(context.Context, int64) (stakingtypes.HistoricalInfo, error) {
	return stakingtypes.HistoricalInfo{}, fmt.Errorf("historical info failed")
}

func (processProposalFailingStakingKeeper) GetValidatorByConsAddr(context.Context, sdk.ConsAddress) (stakingtypes.Validator, error) {
	return stakingtypes.Validator{}, fmt.Errorf("validator not found")
}

type processProposalStatefulKeyregistryKeeper struct {
	cosmosToMina     map[string][]byte
	failGetAfter     int
	overrideGetAfter map[int][]byte
	getCalls         int
}

func (k *processProposalStatefulKeyregistryKeeper) ValidatorCosmosToMinaHas(_ context.Context, cosmosPubKey []byte) (bool, error) {
	_, ok := k.cosmosToMina[string(cosmosPubKey)]
	return ok, nil
}

func (k *processProposalStatefulKeyregistryKeeper) ValidatorGetCosmosToMina(_ context.Context, cosmosPubKey []byte) ([]byte, error) {
	if k.failGetAfter > 0 && k.getCalls >= k.failGetAfter {
		return nil, fmt.Errorf("keyregistry read failed")
	}
	if override, ok := k.overrideGetAfter[k.getCalls]; ok {
		k.getCalls++
		return override, nil
	}

	k.getCalls++
	return k.cosmosToMina[string(cosmosPubKey)], nil
}
