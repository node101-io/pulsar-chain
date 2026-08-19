package abci

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"testing"

	storetypes "cosmossdk.io/store/types"
	cometabci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdked25519 "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	protoio "github.com/cosmos/gogoproto/io"
	"github.com/stretchr/testify/require"

	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
	verificationvalidator "github.com/node101-io/pulsar-chain/x/verification/validator"
)

type verificationKeeperStub struct {
	validateErr error
	applyErr    error
	applied     int
	snapshots   []uint64
}

func (k *verificationKeeperStub) GetProofsAtHeight(context.Context, uint64) ([]verificationtypes.ProofEntry, error) {
	return nil, nil
}

func (k *verificationKeeperStub) GetCommitment(context.Context, []byte, uint64) ([]byte, bool, error) {
	return nil, false, nil
}

type verificationBuilderStub struct {
	outcome verificationvalidator.BuildOutcome
}

func (b verificationBuilderStub) Build(
	context.Context,
	verificationvalidator.Identity,
	uint64,
) verificationvalidator.BuildOutcome {
	return b.outcome
}

func (k *verificationKeeperStub) ValidateVerificationPayload(
	context.Context, []byte, uint64, []byte, []verificationtypes.CommitmentRevelation,
) error {
	return k.validateErr
}

func (k *verificationKeeperStub) ApplyVerificationPayload(
	context.Context, []byte, uint64, []byte, []verificationtypes.CommitmentRevelation,
) error {
	if k.applyErr != nil {
		return k.applyErr
	}
	k.applied++
	return nil
}

func (k *verificationKeeperStub) CreateValidatorSnapshot(_ context.Context, height uint64) error {
	k.snapshots = append(k.snapshots, height)
	return nil
}

func TestValidateVerificationEntriesAuthenticatesCometSigner(t *testing.T) {
	consensusKey, validator := knownConsensusValidator(t, 10)
	secondaryKey := validSecondaryKey()
	keeper := &verificationKeeperStub{}
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, validator)): secondaryKey,
	}, nil)
	handler.verificationKeeper = keeper

	ctx := verificationABCIContext(t, 10)
	body, err := handler.constructVoteExtBody(ctx, 9)
	require.NoError(t, err)
	transitionSignature, err := secondaryKey.SignVoteExtBody(body)
	require.NoError(t, err)
	composite := compositeWithVerification(t, transitionSignature, 10)
	entry := signedVerificationEntry(t, consensusKey, composite, 9, 2, ctx.ChainID())
	payload := Payload{
		VoteExtensionHeight: 9,
		VoteExtensions: []*PayloadVoteExtension{{
			ConsensusPublicKey: consensusPubKeyBytes(t, validator),
			VoteExtension:      transitionSignature,
		}},
		VerificationEntries: []*PayloadVerificationEntry{entry},
	}
	lastCommit := cometabci.CommitInfo{Round: 2, Votes: []cometabci.VoteInfo{{
		Validator:   cometabci.Validator{Address: consensusKey.PubKey().Address(), Power: 10},
		BlockIdFlag: cmtproto.BlockIDFlagCommit,
	}}}

	// Acceptance requires the same historical validator identity to appear in
	// the last commit, the mandatory Mina payload, and the CometBFT-signed
	// composite entry. The test exercises the complete binding instead of proving
	// only that the embedded verification payload is structurally valid.
	actions, err := handler.validateVerificationEntries(ctx, 10, payload, lastCommit)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	require.Equal(t, 1, keeper.applied)

	// Changing only the outer CometBFT signature invalidates the optional action
	// because that signature binds the commitment or revelation to height, round,
	// chain ID, and consensus key. The unchanged inner Mina signature is not
	// sufficient authorization for verification state changes.
	payload.VerificationEntries[0].ExtensionSignature[0] ^= 0xff
	_, err = handler.validateVerificationEntries(ctx, 10, payload, lastCommit)
	require.ErrorIs(t, err, ErrInvalidVerificationSignature)
}

func TestPrepareProposalFiltersInvalidVerificationButKeepsTransitionVote(t *testing.T) {
	consensusKey, validator := knownConsensusValidator(t, 10)
	keeper := &verificationKeeperStub{validateErr: errors.New("invalid optional action")}
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, nil, nil)
	handler.verificationKeeper = keeper
	ctx := verificationABCIContext(t, 10)
	composite := compositeWithVerification(t, []byte("transition-signature"), 10)
	extendedVote := signedExtendedVote(t, consensusKey, composite, 9, 0, ctx.ChainID())

	response, err := handler.PrepareProposalHandler()(ctx, &cometabci.RequestPrepareProposal{
		Height:          10,
		MaxTxBytes:      1 << 20,
		LocalLastCommit: cometabci.ExtendedCommitInfo{Round: 0, Votes: []cometabci.ExtendedVoteInfo{extendedVote}},
	})
	require.NoError(t, err)
	payload, found, err := extractPayload(response.Txs)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, payload.VoteExtensions, 1)
	require.Empty(t, payload.VerificationEntries)
	require.Equal(t, []byte("transition-signature"), payload.VoteExtensions[0].VoteExtension)
}

func TestExtendVoteBuilderFailureKeepsMandatoryTransitionSignature(t *testing.T) {
	_, validator := knownConsensusValidator(t, 10)
	secondaryKey := validSecondaryKey()
	keeper := &verificationKeeperStub{}
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, validator)): secondaryKey,
	}, nil)
	handler.secondaryKey = secondaryKey
	handler.verificationKeeper = keeper
	handler.verificationBuilder = verificationBuilderStub{outcome: verificationvalidator.BuildOutcome{
		Warning: errors.New("sidecar unavailable"),
	}}

	response, err := handler.ExtendVoteHandler()(verificationABCIContext(t, 10), &cometabci.RequestExtendVote{Height: 10})
	require.NoError(t, err)
	composite, err := decodeCompositeVoteExtension(response.VoteExtension)
	require.NoError(t, err)
	require.NotEmpty(t, composite.TransitionSignature)
	require.Nil(t, composite.VerificationPayload)
}

func TestExtendVoteBuilderWarningsKeepMandatoryTransitionSignature(t *testing.T) {
	_, validator := knownConsensusValidator(t, 10)
	secondaryKey := validSecondaryKey()

	for _, testCase := range []struct {
		name    string
		warning error
	}{
		{name: "panic", warning: verificationvalidator.ErrProviderPanic},
		{name: "timeout", warning: context.DeadlineExceeded},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, map[string]SecondaryKey{
				string(consensusPubKeyBytes(t, validator)): secondaryKey,
			}, nil)
			handler.secondaryKey = secondaryKey
			handler.verificationKeeper = &verificationKeeperStub{}
			handler.verificationBuilder = verificationBuilderStub{outcome: verificationvalidator.BuildOutcome{Warning: testCase.warning}}

			response, err := handler.ExtendVoteHandler()(verificationABCIContext(t, 10), &cometabci.RequestExtendVote{Height: 10})
			require.NoError(t, err)
			composite, err := decodeCompositeVoteExtension(response.VoteExtension)
			require.NoError(t, err)
			require.NotEmpty(t, composite.TransitionSignature)
			require.Nil(t, composite.VerificationPayload)
		})
	}
}

func TestExtendVoteRejectsMalformedBuilderPayloadLocally(t *testing.T) {
	_, validator := knownConsensusValidator(t, 10)
	secondaryKey := validSecondaryKey()
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, validator)): secondaryKey,
	}, nil)
	handler.secondaryKey = secondaryKey
	handler.verificationKeeper = &verificationKeeperStub{}
	handler.verificationBuilder = verificationBuilderStub{outcome: verificationvalidator.BuildOutcome{
		Payload: &verificationtypes.VerificationVoteExtensionPayload{
			TargetHeight: 11,
			Commitment:   bytes.Repeat([]byte{1}, verificationtypes.CommitmentHashSize-1),
		},
	}}

	response, err := handler.ExtendVoteHandler()(verificationABCIContext(t, 10), &cometabci.RequestExtendVote{Height: 10})
	require.NoError(t, err)
	composite, err := decodeCompositeVoteExtension(response.VoteExtension)
	require.NoError(t, err)
	require.Nil(t, composite.VerificationPayload)
}

func TestLocalVerificationIdentityBindsLocalKeysToOperator(t *testing.T) {
	_, localValidator := knownConsensusValidator(t, 10)
	_, otherValidator := knownConsensusValidator(t, 10)
	localKey := validSecondaryKey()
	otherKey := secondaryKeyFromSeed(t, [32]byte{2})
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{otherValidator, localValidator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, localValidator)): localKey,
		string(consensusPubKeyBytes(t, otherValidator)): otherKey,
	}, nil)
	handler.secondaryKey = localKey

	identity, err := handler.localVerificationIdentity(verificationABCIContext(t, 10), 10)
	require.NoError(t, err)
	operator, err := sdk.ValAddressFromBech32(localValidator.GetOperator())
	require.NoError(t, err)
	require.Equal(t, "pulsar-test", identity.ChainID)
	require.Equal(t, []byte(operator), identity.OperatorAddress)
	require.Equal(t, consensusPubKeyBytes(t, localValidator), identity.ConsensusPublicKey)
}

func TestLocalVerificationIdentityFailsClosedForNonValidatorKey(t *testing.T) {
	_, validator := knownConsensusValidator(t, 10)
	registeredKey := validSecondaryKey()
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, validator)): registeredKey,
	}, nil)
	handler.secondaryKey = secondaryKeyFromSeed(t, [32]byte{9})

	_, err := handler.localVerificationIdentity(verificationABCIContext(t, 10), 10)
	require.ErrorContains(t, err, "not in validator set")

	handler.secondaryKey = SecondaryKey{}
	_, err = handler.localVerificationIdentity(verificationABCIContext(t, 10), 10)
	require.ErrorIs(t, err, ErrMissingSecondaryKey)
}

func TestValidateVerificationEntriesBindsSignatureToConsensusTuple(t *testing.T) {
	consensusKey, validator := knownConsensusValidator(t, 10)
	secondaryKey := validSecondaryKey()
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, map[string]SecondaryKey{
		string(consensusPubKeyBytes(t, validator)): secondaryKey,
	}, nil)
	handler.verificationKeeper = &verificationKeeperStub{}
	ctx := verificationABCIContext(t, 10)
	body, err := handler.constructVoteExtBody(ctx, 9)
	require.NoError(t, err)
	transitionSignature, err := secondaryKey.SignVoteExtBody(body)
	require.NoError(t, err)
	composite := compositeWithVerification(t, transitionSignature, 10)

	for _, testCase := range []struct {
		name       string
		signHeight int64
		signRound  int32
		signChain  string
	}{
		{name: "wrong height", signHeight: 8, signRound: 2, signChain: ctx.ChainID()},
		{name: "wrong round", signHeight: 9, signRound: 1, signChain: ctx.ChainID()},
		{name: "wrong chain", signHeight: 9, signRound: 2, signChain: "other-chain"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			entry := signedVerificationEntry(t, consensusKey, composite, testCase.signHeight, testCase.signRound, testCase.signChain)
			entry.SourceHeight = 9
			entry.Round = 2
			payload := Payload{
				VoteExtensions: []*PayloadVoteExtension{{
					ConsensusPublicKey: consensusPubKeyBytes(t, validator), VoteExtension: transitionSignature,
				}},
				VerificationEntries: []*PayloadVerificationEntry{entry},
			}
			lastCommit := cometabci.CommitInfo{Round: 2, Votes: []cometabci.VoteInfo{{
				Validator:   cometabci.Validator{Address: consensusKey.PubKey().Address(), Power: 10},
				BlockIdFlag: cmtproto.BlockIDFlagCommit,
			}}}

			_, err := handler.validateVerificationEntries(ctx, 10, payload, lastCommit)
			require.ErrorIs(t, err, ErrInvalidVerificationSignature)
		})
	}
}

func TestValidateVerificationEntriesRejectsDuplicateAndNonCanonicalValidators(t *testing.T) {
	keys := make([]*sdked25519.PrivKey, 2)
	validators := make([]stakingtypes.Validator, 2)
	minaKeys := make(map[string]SecondaryKey, 2)
	for i := range validators {
		keys[i], validators[i] = knownConsensusValidator(t, 10)
		minaKeys[string(consensusPubKeyBytes(t, validators[i]))] = secondaryKeyFromSeed(t, [32]byte{byte(i + 1)})
	}
	handler := newQuorumTestHandler(t, validators, minaKeys, nil)
	handler.verificationKeeper = &verificationKeeperStub{}
	ctx := verificationABCIContext(t, 10)
	entries := make([]*PayloadVerificationEntry, 2)
	mandatory := make([]*PayloadVoteExtension, 2)
	votes := make([]cometabci.VoteInfo, 2)
	for i := range validators {
		body, err := handler.constructVoteExtBody(ctx, 9)
		require.NoError(t, err)
		transition, err := minaKeys[string(consensusPubKeyBytes(t, validators[i]))].SignVoteExtBody(body)
		require.NoError(t, err)
		composite := compositeWithVerification(t, transition, 10)
		entries[i] = signedVerificationEntry(t, keys[i], composite, 9, 0, ctx.ChainID())
		mandatory[i] = &PayloadVoteExtension{ConsensusPublicKey: consensusPubKeyBytes(t, validators[i]), VoteExtension: transition}
		votes[i] = cometabci.VoteInfo{Validator: cometabci.Validator{Address: keys[i].PubKey().Address(), Power: 10}, BlockIdFlag: cmtproto.BlockIDFlagCommit}
	}
	lastCommit := cometabci.CommitInfo{Round: 0, Votes: votes}
	if bytes.Compare(entries[0].ValidatorAddress, entries[1].ValidatorAddress) < 0 {
		entries[0], entries[1] = entries[1], entries[0]
	}
	_, err := handler.validateVerificationEntries(ctx, 10, Payload{VoteExtensions: mandatory, VerificationEntries: entries}, lastCommit)
	require.ErrorIs(t, err, ErrInvalidVerificationPayload)

	duplicate := []*PayloadVerificationEntry{entries[0], entries[0]}
	_, err = handler.validateVerificationEntries(ctx, 10, Payload{VoteExtensions: mandatory, VerificationEntries: duplicate}, lastCommit)
	require.ErrorIs(t, err, ErrInvalidVerificationPayload)
}

func TestVerificationActionErrorsAreProposalRejections(t *testing.T) {
	for _, err := range []error{
		verificationtypes.ErrProofNotFound,
		verificationtypes.ErrProofHeightNotFound,
		verificationtypes.ErrCommitmentMismatch,
		verificationtypes.ErrNonCanonicalVoteOrdering,
	} {
		require.True(t, isInvalidProcessProposalError(err), err.Error())
	}
	require.False(t, isInvalidProcessProposalError(verificationtypes.ErrProofStateCorrupted))
}

func TestVerificationMultiValidatorProposalTransport(t *testing.T) {
	const validatorCount = 3
	keys := make([]*sdked25519.PrivKey, validatorCount)
	validators := make([]stakingtypes.Validator, validatorCount)
	minaKeys := make(map[string]SecondaryKey, validatorCount)
	for i := range validators {
		keys[i], validators[i] = knownConsensusValidator(t, 10)
		minaKeys[string(consensusPubKeyBytes(t, validators[i]))] = secondaryKeyFromSeed(t, [32]byte{byte(i + 1)})
	}
	handler := newQuorumTestHandler(t, validators, minaKeys, nil)
	handler.verificationKeeper = &verificationKeeperStub{}
	ctx := verificationABCIContext(t, 10)
	body, err := handler.constructVoteExtBody(ctx, 9)
	require.NoError(t, err)

	payload := Payload{VoteExtensionHeight: 9}
	lastCommit := cometabci.CommitInfo{Round: 0}
	for i := range validators {
		transition, err := minaKeys[string(consensusPubKeyBytes(t, validators[i]))].SignVoteExtBody(body)
		require.NoError(t, err)
		if i < 2 {
			composite := compositeWithVerification(t, transition, 10)
			payload.VerificationEntries = append(payload.VerificationEntries,
				signedVerificationEntry(t, keys[i], composite, 9, 0, ctx.ChainID()),
			)
		}
		payload.VoteExtensions = append(payload.VoteExtensions, &PayloadVoteExtension{
			ConsensusPublicKey: consensusPubKeyBytes(t, validators[i]),
			VoteExtension:      transition,
		})
		lastCommit.Votes = append(lastCommit.Votes, cometabci.VoteInfo{
			Validator:   cometabci.Validator{Address: keys[i].PubKey().Address(), Power: 10},
			BlockIdFlag: cmtproto.BlockIDFlagCommit,
		})
	}
	sort.Slice(payload.VerificationEntries, func(i, j int) bool {
		return bytes.Compare(payload.VerificationEntries[i].ValidatorAddress, payload.VerificationEntries[j].ValidatorAddress) < 0
	})

	request := &cometabci.RequestProcessProposal{
		Height:             10,
		Txs:                [][]byte{markedPayloadTx(t, payload)},
		ProposedLastCommit: lastCommit,
	}
	response, err := handler.ProcessProposalHandler()(ctx, request)
	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseProcessProposal_ACCEPT, response.Status)

	omitted := payload
	omitted.VerificationEntries = nil
	request.Txs = [][]byte{markedPayloadTx(t, omitted)}
	response, err = handler.ProcessProposalHandler()(ctx, request)
	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseProcessProposal_ACCEPT, response.Status)

	payload.VerificationEntries[0].ExtensionSignature[0] ^= 0xff
	request.Txs = [][]byte{markedPayloadTx(t, payload)}
	response, err = handler.ProcessProposalHandler()(ctx, request)
	require.NoError(t, err)
	require.Equal(t, cometabci.ResponseProcessProposal_REJECT, response.Status)
}

func knownConsensusValidator(t testing.TB, power int64) (*sdked25519.PrivKey, stakingtypes.Validator) {
	t.Helper()
	key := sdked25519.GenPrivKey()
	validator, err := stakingtypes.NewValidator(
		sdk.ValAddress(key.PubKey().Address()).String(), key.PubKey(), stakingtypes.Description{},
	)
	require.NoError(t, err)
	validator = validator.UpdateStatus(stakingtypes.Bonded)
	validator.Tokens = sdk.TokensFromConsensusPower(power, sdk.DefaultPowerReduction)
	return key, validator
}

func verificationABCIContext(t testing.TB, height int64) sdk.Context {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey("verification_abci_test")
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("verification_abci_transient")).Ctx
	return ctx.WithBlockHeight(height).WithChainID("pulsar-test").WithConsensusParams(cmtproto.ConsensusParams{
		Abci: &cmtproto.ABCIParams{VoteExtensionsEnableHeight: 1},
	})
}

func compositeWithVerification(t testing.TB, transitionSignature []byte, targetHeight uint64) []byte {
	t.Helper()
	encoded, err := encodeCompositeVoteExtension(&CompositeVoteExtension{
		ProtocolVersion:     CompositeVoteExtensionVersion,
		TransitionSignature: append([]byte(nil), transitionSignature...),
		VerificationPayload: &verificationtypes.VerificationVoteExtensionPayload{
			TargetHeight: targetHeight,
			Commitment:   bytes.Repeat([]byte{7}, verificationtypes.CommitmentHashSize),
		},
	})
	require.NoError(t, err)
	return encoded
}

func signedVerificationEntry(
	t testing.TB,
	key *sdked25519.PrivKey,
	extension []byte,
	height int64,
	round int32,
	chainID string,
) *PayloadVerificationEntry {
	t.Helper()
	vote := signedExtendedVote(t, key, extension, height, round, chainID)
	return &PayloadVerificationEntry{
		ValidatorAddress:       append([]byte(nil), vote.Validator.Address...),
		SourceHeight:           height,
		Round:                  round,
		CompositeVoteExtension: append([]byte(nil), extension...),
		ExtensionSignature:     append([]byte(nil), vote.ExtensionSignature...),
	}
}

func signedExtendedVote(
	t testing.TB,
	key *sdked25519.PrivKey,
	extension []byte,
	height int64,
	round int32,
	chainID string,
) cometabci.ExtendedVoteInfo {
	t.Helper()
	canonical := cmtproto.CanonicalVoteExtension{Extension: extension, Height: height, Round: int64(round), ChainId: chainID}
	var signBytes bytes.Buffer
	require.NoError(t, protoio.NewDelimitedWriter(&signBytes).WriteMsg(&canonical))
	signature, err := key.Sign(signBytes.Bytes())
	require.NoError(t, err)
	return cometabci.ExtendedVoteInfo{
		Validator:          cometabci.Validator{Address: key.PubKey().Address(), Power: 10},
		VoteExtension:      append([]byte(nil), extension...),
		ExtensionSignature: signature,
		BlockIdFlag:        cmtproto.BlockIDFlagCommit,
	}
}
