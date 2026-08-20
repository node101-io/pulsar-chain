package abci

import (
	"context"
	"math"
	"testing"

	cometabci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/privatekey"
	keyregistrytypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
	verificationvalidator "github.com/node101-io/pulsar-chain/x/verification/validator"
	votepersistencetypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
	"github.com/stretchr/testify/require"
)

func TestValidatePayloadVoteExtensionsReturnsVerifiedVotesAndPower(t *testing.T) {
	firstValidator := newTestBondedValidator(t, 10)
	secondValidator := newTestBondedValidator(t, 5)
	firstMinaKey := validSecondaryKey()
	secondMinaKey := secondaryKeyFromSeed(t, [32]byte{2})
	body := validVoteExtBody()
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{firstValidator, secondValidator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, firstValidator)):  firstMinaKey,
		string(consensusPubKeyBytes(t, secondValidator)): secondMinaKey,
	}, nil)
	payload := Payload{VoteExtensions: []*PayloadVoteExtension{
		signedPayloadVoteExtension(t, firstValidator, firstMinaKey, body),
		signedPayloadVoteExtension(t, secondValidator, secondMinaKey, body),
	}}

	verifiedVotes, err := handler.validatePayloadVoteExtensions(sdk.Context{}, 10, payload, body)

	require.NoError(t, err)
	require.Len(t, verifiedVotes.votes, 2)
	require.Equal(t, int64(15), verifiedVotes.signedPower)
	require.Equal(t, int64(15), verifiedVotes.totalPower)
	require.Equal(t, consensusPubKeyBytes(t, firstValidator), verifiedVotes.votes[0].consensusPublicKey)
	require.Equal(t, int64(10), verifiedVotes.votes[0].power)
	require.Equal(t, testMinaPublicKeyFromSecondaryKey(t, firstMinaKey), verifiedVotes.votes[0].minaPublicKey)
}

func TestValidatePayloadVoteExtensionsUsesFirstDuplicateVote(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	secondaryKey := validSecondaryKey()
	body := validVoteExtBody()
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, validator)): secondaryKey,
	}, nil)
	firstVote := signedPayloadVoteExtension(t, validator, secondaryKey, body)
	secondVote := &PayloadVoteExtension{
		ConsensusPublicKey: firstVote.ConsensusPublicKey,
		VoteExtension:      []byte("invalid-duplicate-vote"),
	}

	verifiedVotes, err := handler.validatePayloadVoteExtensions(sdk.Context{}, 10, Payload{VoteExtensions: []*PayloadVoteExtension{firstVote, secondVote}}, body)

	require.NoError(t, err)
	require.Len(t, verifiedVotes.votes, 1)
	require.Equal(t, firstVote.VoteExtension, verifiedVotes.votes[0].voteExtension)
}

func TestValidatePayloadVoteExtensionsRejectsInvalidFirstDuplicate(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	secondaryKey := validSecondaryKey()
	body := validVoteExtBody()
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, validator)): secondaryKey,
	}, nil)
	validVote := signedPayloadVoteExtension(t, validator, secondaryKey, body)
	invalidFirstVote := &PayloadVoteExtension{
		ConsensusPublicKey: validVote.ConsensusPublicKey,
		VoteExtension:      []byte("invalid-first-duplicate"),
	}

	_, err := handler.validatePayloadVoteExtensions(sdk.Context{}, 10, Payload{VoteExtensions: []*PayloadVoteExtension{invalidFirstVote, validVote}}, body)

	require.ErrorIs(t, err, votepersistencetypes.ErrInvalidVoteExtension)
}

func TestValidatePayloadVoteExtensionsRejectsMissingMinaKey(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	secondaryKey := validSecondaryKey()
	body := validVoteExtBody()
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, nil, nil)
	payload := Payload{VoteExtensions: []*PayloadVoteExtension{signedPayloadVoteExtension(t, validator, secondaryKey, body)}}

	_, err := handler.validatePayloadVoteExtensions(sdk.Context{}, 10, payload, body)

	require.ErrorIs(t, err, keyregistrytypes.ErrValidatorNotRegistered)
}

func TestValidatePayloadVoteExtensionsRejectsInvalidSignature(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	secondaryKey := validSecondaryKey()
	body := validVoteExtBody()
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, validator)): secondaryKey,
	}, nil)
	payload := Payload{VoteExtensions: []*PayloadVoteExtension{{
		ConsensusPublicKey: consensusPubKeyBytes(t, validator),
		VoteExtension:      []byte("invalid-signature"),
	}}}

	_, err := handler.validatePayloadVoteExtensions(sdk.Context{}, 10, payload, body)

	require.ErrorIs(t, err, votepersistencetypes.ErrInvalidVoteExtension)
}

func TestHasAtLeastTwoThirdsPower(t *testing.T) {
	require.True(t, hasAtLeastTwoThirdsPower(2, 3))
	require.True(t, hasAtLeastTwoThirdsPower(4, 6))
	require.False(t, hasAtLeastTwoThirdsPower(1, 3))
	require.False(t, hasAtLeastTwoThirdsPower(1, 0))
	require.True(t, hasAtLeastTwoThirdsPower(math.MaxInt64, math.MaxInt64))
	require.False(t, hasAtLeastTwoThirdsPower(math.MaxInt64/2, math.MaxInt64))
}

func TestPreBlockerPersistsOnlyCanonicalVerifiedVotes(t *testing.T) {
	reqHeight := int64(10)
	validator := newTestBondedValidator(t, 10)
	secondaryKey := validSecondaryKey()
	votePersistenceKeeper := &quorumTestVotePersistenceKeeper{}
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, validator)): secondaryKey,
	}, votePersistenceKeeper)
	ctx := verificationABCIContext(t, reqHeight)
	body, err := handler.constructVoteExtBody(ctx, reqHeight-1)
	require.NoError(t, err)
	firstVote := signedPayloadVoteExtension(t, validator, secondaryKey, body)
	secondVote := &PayloadVoteExtension{
		ConsensusPublicKey: firstVote.ConsensusPublicKey,
		VoteExtension:      []byte("invalid-duplicate-vote"),
	}

	response, err := handler.PreBlocker()(ctx, &cometabci.RequestFinalizeBlock{
		Height: reqHeight,
		Txs: [][]byte{markedPayloadTx(t, Payload{
			VoteExtensionHeight: reqHeight - 1,
			VoteExtensions:      []*PayloadVoteExtension{firstVote, secondVote},
		})},
	})

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, 1, votePersistenceKeeper.clearCalls)
	require.Len(t, votePersistenceKeeper.setVotes, 1)
	require.Equal(t, reqHeight-3, votePersistenceKeeper.setVotes[0].height)
	require.Equal(t, testMinaPublicKeyFromSecondaryKey(t, secondaryKey), votePersistenceKeeper.setVotes[0].minaPublicKey)
	require.Equal(t, firstVote.VoteExtension, votePersistenceKeeper.setVotes[0].voteExtension)
}

func TestPreBlockerDoesNotMutateStoreWhenValidationFails(t *testing.T) {
	reqHeight := int64(10)
	validator := newTestBondedValidator(t, 10)
	secondaryKey := validSecondaryKey()
	votePersistenceKeeper := &quorumTestVotePersistenceKeeper{}
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, validator)): secondaryKey,
	}, votePersistenceKeeper)
	ctx := verificationABCIContext(t, reqHeight)
	payload := Payload{
		VoteExtensionHeight: reqHeight - 1,
		VoteExtensions: []*PayloadVoteExtension{{
			ConsensusPublicKey: consensusPubKeyBytes(t, validator),
			VoteExtension:      []byte("invalid-signature"),
		}},
	}

	response, err := handler.PreBlocker()(ctx, &cometabci.RequestFinalizeBlock{
		Height: reqHeight,
		Txs:    [][]byte{markedPayloadTx(t, payload)},
	})

	require.Nil(t, response)
	require.ErrorIs(t, err, votepersistencetypes.ErrInvalidVoteExtension)
	require.Zero(t, votePersistenceKeeper.clearCalls)
	require.Empty(t, votePersistenceKeeper.setVotes)
}

func newQuorumTestHandler(t *testing.T, validators []stakingtypes.Validator, keyByConsensusPubKey map[string]SecondaryKey, votePersistenceKeeper VotePersistenceKeeper) *ABCIHandler {
	t.Helper()

	cosmosToMina := make(map[string][]byte, len(keyByConsensusPubKey))
	for consensusPubKey, secondaryKey := range keyByConsensusPubKey {
		cosmosToMina[consensusPubKey] = secondaryKey.PublicKey.Bytes()
	}

	if votePersistenceKeeper == nil {
		votePersistenceKeeper = &quorumTestVotePersistenceKeeper{}
	}

	return &ABCIHandler{
		stakingKeeper: quorumTestStakingKeeper{
			validators:           validators,
			validatorsByConsAddr: validatorsByConsAddr(t, validators...),
		},
		keyregistryKeeper:     quorumTestKeyregistryKeeper{cosmosToMina: cosmosToMina},
		votePersistenceKeeper: votePersistenceKeeper,
		networkID:             NetworkID,
		bridgeKeeper:          testBridgeKeeper{},
		verificationKeeper:    &verificationKeeperStub{},
		verificationBuilder:   verificationvalidator.DisabledBuilder{},
	}
}

func signedPayloadVoteExtension(t *testing.T, validator stakingtypes.Validator, secondaryKey SecondaryKey, body votepersistencetypes.VoteExtBody) *PayloadVoteExtension {
	t.Helper()

	signature, err := secondaryKey.SignVoteExtBody(body)
	require.NoError(t, err)

	return &PayloadVoteExtension{
		ConsensusPublicKey: consensusPubKeyBytes(t, validator),
		VoteExtension:      signature,
	}
}

func secondaryKeyFromSeed(t *testing.T, seed [32]byte) SecondaryKey {
	t.Helper()

	privateKey, err := privatekey.NewPrivateKeyFromBytes(seed, NetworkID)
	require.NoError(t, err)
	publicKey, err := privateKey.ToPublicKey()
	require.NoError(t, err)

	return SecondaryKey{
		SecretKey: privateKey,
		PublicKey: publicKey,
	}
}

func testMinaPublicKeyFromSecondaryKey(t *testing.T, secondaryKey SecondaryKey) []byte {
	t.Helper()

	return secondaryKey.PublicKey.Bytes()
}

func markedPayloadTx(t *testing.T, payload Payload) []byte {
	t.Helper()

	payloadBytes, err := payload.Marshal()
	require.NoError(t, err)

	return append(voteExtMarkerBytes[:len(voteExtMarkerBytes):len(voteExtMarkerBytes)], payloadBytes...)
}

type quorumTestStakingKeeper struct {
	validators           []stakingtypes.Validator
	validatorsByConsAddr map[string]stakingtypes.Validator
}

func (k quorumTestStakingKeeper) IterateLastValidators(_ context.Context, fn func(int64, stakingtypes.ValidatorI) bool) error {
	for i, validator := range k.validators {
		if fn(int64(i), validator) {
			break
		}
	}

	return nil
}

func (k quorumTestStakingKeeper) GetHistoricalInfo(context.Context, int64) (stakingtypes.HistoricalInfo, error) {
	return stakingtypes.HistoricalInfo{
		Header: tmproto.Header{AppHash: testStateRoot32()},
		Valset: k.validators,
	}, nil
}

func (k quorumTestStakingKeeper) GetValidatorByConsAddr(_ context.Context, consAddr sdk.ConsAddress) (stakingtypes.Validator, error) {
	validator, ok := k.validatorsByConsAddr[string(consAddr)]
	if !ok {
		return stakingtypes.Validator{}, ErrInvalidPayload
	}

	return validator, nil
}

type quorumTestKeyregistryKeeper struct {
	cosmosToMina map[string][]byte
}

func (k quorumTestKeyregistryKeeper) ValidatorCosmosToMinaHas(_ context.Context, cosmosPubKey []byte) (bool, error) {
	_, ok := k.cosmosToMina[string(cosmosPubKey)]
	return ok, nil
}

func (k quorumTestKeyregistryKeeper) ValidatorGetCosmosToMina(_ context.Context, cosmosPubKey []byte) ([]byte, error) {
	return k.cosmosToMina[string(cosmosPubKey)], nil
}

type quorumTestVotePersistenceKeeper struct {
	clearCalls int
	setVotes   []quorumTestPersistedVote
}

func (k *quorumTestVotePersistenceKeeper) Clear(context.Context) error {
	k.clearCalls++
	return nil
}

func (k *quorumTestVotePersistenceKeeper) SetVote(_ context.Context, height int64, minaPublicKey, voteExtension []byte) error {
	k.setVotes = append(k.setVotes, quorumTestPersistedVote{
		height:        height,
		minaPublicKey: minaPublicKey,
		voteExtension: voteExtension,
	})
	return nil
}

type quorumTestPersistedVote struct {
	height        int64
	minaPublicKey []byte
	voteExtension []byte
}
