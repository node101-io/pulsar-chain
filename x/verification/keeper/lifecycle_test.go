package keeper_test

import (
	"encoding/hex"
	"testing"

	comettypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func TestGenesisRoundTripPreservesActiveState(t *testing.T) {
	source := initFixture(t, 3)
	require.NoError(t, source.keeper.Params.Set(source.ctx, types.NewParams(42)))
	submitProof(t, source, 500, 1)
	require.NoError(t, source.keeper.EndBlock(source.atHeight(500)))

	exported, err := source.keeper.ExportGenesis(source.ctx)
	require.NoError(t, err)
	require.Equal(t, types.NewParams(42), exported.Params)
	require.NoError(t, exported.Validate())
	require.Len(t, exported.PendingProofs, 1)
	require.Len(t, exported.SeenVerificationIds, 1)
	require.NoError(t, types.ValidateProofRecord(exported.PendingProofs[0].Proof))
	require.Equal(t, exported.PendingProofs[0].Proof.VerificationId, exported.SeenVerificationIds[0].VerificationId)
	require.Len(t, exported.ValidatorPowers, 3)
	require.Equal(t, int64(100), exported.TotalVotingPowers[0].TotalVotingPower)

	target := initFixture(t, 3)
	require.NoError(t, target.keeper.InitGenesis(target.ctx, *exported))
	params, err := target.keeper.Params.Get(target.ctx)
	require.NoError(t, err)
	require.Equal(t, types.NewParams(42), params)
	pending, err := target.keeper.PendingProofs.Has(target.ctx, types.NewProofStoreKey(500, 0))
	require.NoError(t, err)
	require.True(t, pending)
	seen, err := target.keeper.SeenVerificationIDs.Has(target.ctx, exported.SeenVerificationIds[0].VerificationId)
	require.NoError(t, err)
	require.True(t, seen)
}

func TestGenesisRoundTripPreservesFinalizedState(t *testing.T) {
	source := initFixture(t, 1)
	submitProof(t, source, 500, 1)
	require.NoError(t, source.keeper.EndBlock(source.atHeight(500)))
	left := valueLeaf(t, 1, []types.ProofVote{{IndexInBlock: 0, Result: true}})
	right := valueLeaf(t, 2, nil)
	require.NoError(t, source.keeper.ApplyVerificationPayload(
		source.atHeight(503), source.validators[0].operator, 503, commitmentFor(t, left, right), nil,
	))
	require.NoError(t, source.keeper.ApplyVerificationPayload(
		source.atHeight(504), source.validators[0].operator, 504, nil,
		[]types.CommitmentRevelation{{CommitmentHeight: 503, Left: left, Right: hashLeaf(t, right)}},
	))
	require.NoError(t, source.keeper.EndBlock(source.atHeight(505)))

	exported, err := source.keeper.ExportGenesis(source.ctx)
	require.NoError(t, err)
	require.NoError(t, exported.Validate())
	require.Empty(t, exported.PendingProofs)
	require.Len(t, exported.FinalProofResults, 1)
	require.Len(t, exported.SeenVerificationIds, 1)
	final := exported.FinalProofResults[0].Result
	require.NotEmpty(t, final.ProofHash)
	require.NotEmpty(t, final.PublicInputsHash)
	require.NotEmpty(t, final.VerificationKeyHash)
	require.Equal(t, exported.SeenVerificationIds[0].VerificationId, final.VerificationId)

	target := initFixture(t, 1)
	require.NoError(t, target.keeper.InitGenesis(target.ctx, *exported))
	response, err := target.query.ProofByVerificationId(target.ctx, &types.QueryProofByVerificationIdRequest{
		VerificationId: exported.SeenVerificationIds[0].VerificationId,
	})
	require.NoError(t, err)
	require.Equal(t, types.ProofStatus_PROOF_STATUS_VALID, response.GetFinalResult().Status)
}

func TestLifecycleEventsUseCanonicalAttributes(t *testing.T) {
	fixture := initFixture(t, 3)
	msg := proofSubmission(fixture, 1)
	verificationID, err := types.ComputeVerificationID(msg.ProofType, msg.ProofHash, msg.PublicInputsHash, msg.VerificationKeyHash)
	require.NoError(t, err)
	proofCtx := fixture.atHeight(500)
	_, err = fixture.msgServer.SubmitProof(proofCtx, msg)
	require.NoError(t, err)
	require.NoError(t, fixture.keeper.EndBlock(fixture.atHeight(500)))
	require.Equal(t, map[string]string{
		types.AttributeKeyVerificationID:      hex.EncodeToString(verificationID[:]),
		types.AttributeKeyProofHash:           hex.EncodeToString(msg.ProofHash),
		types.AttributeKeyPublicInputsHash:    hex.EncodeToString(msg.PublicInputsHash),
		types.AttributeKeyVerificationKeyHash: hex.EncodeToString(msg.VerificationKeyHash),
		types.AttributeKeySubmissionHeight:    "500",
		types.AttributeKeyIndexInBlock:        "0",
		types.AttributeKeyProofType:           "1",
	}, attributesForEvent(t, proofCtx, types.EventTypeProofSubmitted))

	left := valueLeaf(t, 1, []types.ProofVote{{IndexInBlock: 0, Result: true}})
	right := valueLeaf(t, 2, nil)
	commitment := commitmentFor(t, left, right)
	commitmentCtx := fixture.atHeight(503)
	err = fixture.keeper.ApplyVerificationPayload(
		commitmentCtx, fixture.validators[0].operator, 503, commitment, nil,
	)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		types.AttributeKeyValidator:        fixture.validators[0].operatorS,
		types.AttributeKeyCommitmentHeight: "503",
		types.AttributeKeyCommitment:       hex.EncodeToString(commitment),
	}, attributesForEvent(t, commitmentCtx, types.EventTypeCommitmentSubmitted))

	revealCtx := fixture.atHeight(504)
	err = fixture.keeper.ApplyVerificationPayload(
		revealCtx, fixture.validators[0].operator, 504, nil,
		[]types.CommitmentRevelation{{
			CommitmentHeight: 503, Left: left, Right: hashLeaf(t, right),
		}},
	)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		types.AttributeKeyValidator:        fixture.validators[0].operatorS,
		types.AttributeKeyCommitmentHeight: "503",
	}, attributesForEvent(t, revealCtx, types.EventTypeCommitmentRevealed))

	finalizeCtx := fixture.atHeight(505)
	require.NoError(t, fixture.keeper.EndBlock(finalizeCtx))
	require.Equal(t, map[string]string{
		types.AttributeKeyVerificationID:       hex.EncodeToString(verificationID[:]),
		types.AttributeKeyProofHash:            hex.EncodeToString(msg.ProofHash),
		types.AttributeKeyPublicInputsHash:     hex.EncodeToString(msg.PublicInputsHash),
		types.AttributeKeyVerificationKeyHash:  hex.EncodeToString(msg.VerificationKeyHash),
		types.AttributeKeyProofType:            "1",
		types.AttributeKeySubmissionHeight:     "500",
		types.AttributeKeyIndexInBlock:         "0",
		types.AttributeKeyStatus:               types.ProofStatus_PROOF_STATUS_INCONCLUSIVE.String(),
		types.AttributeKeyValidVotingPower:     "60",
		types.AttributeKeyInvalidVotingPower:   "0",
		types.AttributeKeyTotalVotingPower:     "100",
		types.AttributeKeyVotingPowerThreshold: "67",
	}, attributesForEvent(t, finalizeCtx, types.EventTypeProofFinalized))
}

func attributesForEvent(t testing.TB, ctx sdk.Context, eventType string) map[string]string {
	t.Helper()
	for _, event := range ctx.EventManager().Events() {
		if event.Type != eventType {
			continue
		}
		attributes := make(map[string]string, len(event.Attributes))
		for _, attribute := range event.Attributes {
			attributes[attribute.Key] = attribute.Value
		}
		return attributes
	}
	t.Fatalf("event %q not found", eventType)
	return nil
}

func TestSubmitProofDefersPowerMaterializationAndRejectsDuplicate(t *testing.T) {
	f := initFixture(t, 3)
	first := submitProof(t, f, 500, 1)
	second := submitProof(t, f, 500, 2)
	require.Equal(t, uint32(0), first.IndexInBlock)
	require.Equal(t, uint32(1), second.IndexInBlock)

	exists, err := f.keeper.TotalVotingPowerByHeight.Has(f.ctx, 500)
	require.NoError(t, err)
	require.False(t, exists)
	require.NoError(t, f.keeper.EndBlock(f.atHeight(500)))
	totalPower, err := f.keeper.TotalVotingPowerByHeight.Get(f.ctx, 500)
	require.NoError(t, err)
	require.Equal(t, int64(100), totalPower)

	_, err = f.msgServer.SubmitProof(f.atHeight(501), proofSubmission(f, 1))
	require.ErrorIs(t, err, types.ErrDuplicateVerificationRequest)
}

func TestSubmitProofIdentityAllowsSameProofWithDifferentDescriptor(t *testing.T) {
	f := initFixture(t, 3)
	first := proofSubmission(f, 1)
	firstResponse, err := f.msgServer.SubmitProof(f.atHeight(500), first)
	require.NoError(t, err)

	differentInputs := proofSubmission(f, 1)
	differentInputs.PublicInputsHash[0] = 0x99
	secondResponse, err := f.msgServer.SubmitProof(f.atHeight(500), differentInputs)
	require.NoError(t, err)
	differentKey := proofSubmission(f, 1)
	differentKey.VerificationKeyHash[0] = 0x88
	thirdResponse, err := f.msgServer.SubmitProof(f.atHeight(500), differentKey)
	require.NoError(t, err)
	require.Equal(t, uint32(0), firstResponse.IndexInBlock)
	require.Equal(t, uint32(1), secondResponse.IndexInBlock)
	require.Equal(t, uint32(2), thirdResponse.IndexInBlock)

	firstRecord, err := f.keeper.PendingProofs.Get(f.ctx, types.NewProofStoreKey(500, 0))
	require.NoError(t, err)
	secondRecord, err := f.keeper.PendingProofs.Get(f.ctx, types.NewProofStoreKey(500, 1))
	require.NoError(t, err)
	thirdRecord, err := f.keeper.PendingProofs.Get(f.ctx, types.NewProofStoreKey(500, 2))
	require.NoError(t, err)
	require.Equal(t, firstRecord.ProofHash, secondRecord.ProofHash)
	require.Equal(t, firstRecord.ProofHash, thirdRecord.ProofHash)
	require.NotEqual(t, firstRecord.VerificationId, secondRecord.VerificationId)
	require.NotEqual(t, firstRecord.VerificationId, thirdRecord.VerificationId)

	pending, err := f.query.ProofByVerificationId(f.ctx, &types.QueryProofByVerificationIdRequest{
		VerificationId: secondRecord.VerificationId,
	})
	require.NoError(t, err)
	require.Equal(t, secondRecord, *pending.GetPending())

	_, err = f.msgServer.SubmitProof(f.atHeight(501), differentInputs)
	require.ErrorIs(t, err, types.ErrDuplicateVerificationRequest)
}

func TestEndBlockMaterializesHistoricalPowerForProofHeight(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 500, 1)
	require.NoError(t, f.keeper.EndBlock(f.atHeight(500)))

	totalPower, err := f.keeper.TotalVotingPowerByHeight.Get(f.ctx, 500)
	require.NoError(t, err)
	require.Equal(t, int64(100), totalPower)
	for _, validator := range f.validators {
		power, err := f.keeper.ValidatorPowers.Get(f.ctx, types.NewValidatorPowerStoreKey(500, validator.operator))
		require.NoError(t, err)
		require.Equal(t, validator.power, power)
	}
}

func TestMaterializeValidatorPowersIsIdempotent(t *testing.T) {
	f := initFixture(t, 3)
	require.NoError(t, f.keeper.MaterializeValidatorPowers(f.atHeight(500), 500))
	info := f.staking.historicalInfo[500]
	info.Valset = info.Valset[:2]
	f.staking.historicalInfo[500] = info
	require.NoError(t, f.keeper.MaterializeValidatorPowers(f.atHeight(500), 500))

	totalPower, err := f.keeper.TotalVotingPowerByHeight.Get(f.ctx, 500)
	require.NoError(t, err)
	require.Equal(t, int64(100), totalPower)
	for _, validator := range f.validators {
		exists, err := f.keeper.ValidatorPowers.Has(f.ctx, types.NewValidatorPowerStoreKey(500, validator.operator))
		require.NoError(t, err)
		require.True(t, exists)
	}
}

func TestMaterializeValidatorPowersRejectsMalformedHistoryAtomically(t *testing.T) {
	testCases := []struct {
		name  string
		setup func(*fixture)
	}{
		{
			name: "missing history",
			setup: func(f *fixture) {
				delete(f.staking.historicalInfo, 500)
			},
		},
		{
			name: "wrong header height",
			setup: func(f *fixture) {
				info := f.staking.historicalInfo[500]
				info.Header.Height = 499
				f.staking.historicalInfo[500] = info
			},
		},
		{
			name: "empty validator set",
			setup: func(f *fixture) {
				info := f.staking.historicalInfo[500]
				info.Valset = nil
				f.staking.historicalInfo[500] = info
			},
		},
		{
			name: "duplicate validator",
			setup: func(f *fixture) {
				info := f.staking.historicalInfo[500]
				info.Valset[1] = info.Valset[0]
				f.staking.historicalInfo[500] = info
			},
		},
		{
			name: "invalid operator address",
			setup: func(f *fixture) {
				info := f.staking.historicalInfo[500]
				info.Valset[0].OperatorAddress = "not-a-validator-address"
				f.staking.historicalInfo[500] = info
			},
		},
		{
			name: "zero consensus power",
			setup: func(f *fixture) {
				info := f.staking.historicalInfo[500]
				info.Valset[0].Status = 0
				f.staking.historicalInfo[500] = info
			},
		},
		{
			name: "negative consensus power",
			setup: func(f *fixture) {
				info := f.staking.historicalInfo[500]
				info.Valset[0].Tokens = sdk.TokensFromConsensusPower(-1, sdk.DefaultPowerReduction)
				f.staking.historicalInfo[500] = info
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := initFixture(t, 3)
			submitProof(t, f, 500, 1)
			testCase.setup(f)
			require.Error(t, f.keeper.EndBlock(f.atHeight(500)))

			exists, err := f.keeper.TotalVotingPowerByHeight.Has(f.ctx, 500)
			require.NoError(t, err)
			require.False(t, exists)
			for _, validator := range f.validators {
				exists, err = f.keeper.ValidatorPowers.Has(f.ctx, types.NewValidatorPowerStoreKey(500, validator.operator))
				require.NoError(t, err)
				require.False(t, exists)
			}
		})
	}
}

func TestMaterializeValidatorPowersRejectsTotalPowerOverflow(t *testing.T) {
	f := initFixtureWithPowers(t, []int64{comettypes.MaxTotalVotingPower, 1})
	submitProof(t, f, 500, 1)
	err := f.keeper.EndBlock(f.atHeight(500))
	require.ErrorIs(t, err, types.ErrProofStateCorrupted)
	exists, err := f.keeper.TotalVotingPowerByHeight.Has(f.ctx, 500)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestMaterializeValidatorPowersRejectsCorruptedStoredSnapshot(t *testing.T) {
	f := initFixture(t, 3)
	require.NoError(t, f.keeper.ValidatorPowers.Set(
		f.ctx,
		types.NewValidatorPowerStoreKey(500, f.validators[0].operator),
		f.validators[0].power,
	))
	require.NoError(t, f.keeper.TotalVotingPowerByHeight.Set(f.ctx, 500, 100))
	require.ErrorIs(t, f.keeper.MaterializeValidatorPowers(f.ctx, 500), types.ErrProofStateCorrupted)
}

func TestValidatorPowerAtHeightDistinguishesMembershipFromCorruption(t *testing.T) {
	f := initFixture(t, 3)
	require.NoError(t, f.keeper.MaterializeValidatorPowers(f.ctx, 500))

	power, totalPower, err := f.keeper.ValidatorPowerAtHeight(f.ctx, 500, f.validators[0].operator)
	require.NoError(t, err)
	require.Equal(t, int64(60), power)
	require.Equal(t, int64(100), totalPower)

	unknownValidator := make([]byte, len(f.validators[0].operator))
	unknownValidator[len(unknownValidator)-1] = 99
	_, _, err = f.keeper.ValidatorPowerAtHeight(f.ctx, 500, unknownValidator)
	require.ErrorIs(t, err, types.ErrInvalidValidator)

	require.NoError(t, f.keeper.ValidatorPowers.Set(
		f.ctx,
		types.NewValidatorPowerStoreKey(500, f.validators[0].operator),
		0,
	))
	_, _, err = f.keeper.ValidatorPowerAtHeight(f.ctx, 500, f.validators[0].operator)
	require.ErrorIs(t, err, types.ErrProofStateCorrupted)

	_, _, err = f.keeper.ValidatorPowerAtHeight(f.ctx, 501, f.validators[0].operator)
	require.ErrorIs(t, err, types.ErrProofStateCorrupted)
}

func TestEndBlockWithoutProofDoesNotMaterializePower(t *testing.T) {
	f := initFixture(t, 3)
	require.NoError(t, f.keeper.EndBlock(f.atHeight(500)))

	exists, err := f.keeper.TotalVotingPowerByHeight.Has(f.ctx, 500)
	require.NoError(t, err)
	require.False(t, exists)
	for _, validator := range f.validators {
		exists, err = f.keeper.ValidatorPowers.Has(f.ctx, types.NewValidatorPowerStoreKey(500, validator.operator))
		require.NoError(t, err)
		require.False(t, exists)
	}
}

func TestEndBlockRejectsEmptyHistoricalValidatorSetWithoutPowerWrites(t *testing.T) {
	f := initFixture(t, 0)
	_, err := f.msgServer.SubmitProof(f.atHeight(500), proofSubmission(f, 1))
	require.NoError(t, err)
	err = f.keeper.EndBlock(f.atHeight(500))
	require.ErrorIs(t, err, types.ErrEmptyValidatorSet)
	exists, err := f.keeper.TotalVotingPowerByHeight.Has(f.ctx, 500)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestCommitmentWritesPrimaryAndReverseIndex(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 500, 1)
	require.NoError(t, f.keeper.EndBlock(f.atHeight(500)))
	commitment := make([]byte, types.CommitmentHashSize)
	commitment[0] = 1
	err := f.keeper.ApplyVerificationPayload(f.atHeight(502), f.validators[0].operator, 502, commitment, nil)
	require.NoError(t, err)

	primary, err := f.keeper.Commitments.Has(f.ctx, types.NewCommitmentStoreKey(f.validators[0].operator, 502))
	require.NoError(t, err)
	require.True(t, primary)
	reverse, err := f.keeper.CommitmentsByHeight.Has(f.ctx, types.NewCommitmentHeightStoreKey(502, f.validators[0].operator))
	require.NoError(t, err)
	require.True(t, reverse)

	err = f.keeper.ApplyVerificationPayload(f.atHeight(502), f.validators[0].operator, 502, commitment, nil)
	require.ErrorIs(t, err, types.ErrCommitmentAlreadyExists)
}

func TestRevealBatchStagesEquivocationAtomically(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 500, 1)
	require.NoError(t, f.keeper.EndBlock(f.atHeight(500)))

	left502 := valueLeaf(t, 1, nil)
	right502 := valueLeaf(t, 2, []types.ProofVote{{IndexInBlock: 0, Result: true}})
	root502 := commitmentFor(t, left502, right502)
	err := f.keeper.ApplyVerificationPayload(f.atHeight(502), f.validators[0].operator, 502, root502, nil)
	require.NoError(t, err)

	left503 := valueLeaf(t, 3, []types.ProofVote{{IndexInBlock: 0, Result: false}})
	right503 := valueLeaf(t, 4, nil)
	root503 := commitmentFor(t, left503, right503)
	err = f.keeper.ApplyVerificationPayload(f.atHeight(503), f.validators[0].operator, 503, root503, nil)
	require.NoError(t, err)

	err = f.keeper.ApplyVerificationPayload(
		f.atHeight(504), f.validators[0].operator, 504, nil,
		[]types.CommitmentRevelation{
			{CommitmentHeight: 502, Left: left502, Right: right502},
			{CommitmentHeight: 503, Left: left503, Right: hashLeaf(t, right503)},
		},
	)
	require.NoError(t, err)

	vote, err := f.keeper.VerificationVotes.Get(f.ctx, types.NewVerificationVoteStoreKey(500, 0, f.validators[0].operator))
	require.NoError(t, err)
	require.Equal(t, uint32(types.VoteState_VOTE_STATE_EQUIVOCATED), vote)
	tally, err := f.keeper.ProofTallies.Get(f.ctx, types.NewProofStoreKey(500, 0))
	require.NoError(t, err)
	require.Equal(t, types.ProofTally{}, tally)
}

func TestRevealBatchInvalidLaterEntryLeavesNoVotes(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 500, 1)
	require.NoError(t, f.keeper.EndBlock(f.atHeight(500)))
	left := valueLeaf(t, 1, []types.ProofVote{{IndexInBlock: 0, Result: true}})
	right := valueLeaf(t, 2, nil)
	root := commitmentFor(t, left, right)
	err := f.keeper.ApplyVerificationPayload(f.atHeight(503), f.validators[0].operator, 503, root, nil)
	require.NoError(t, err)

	badRight := hashLeaf(t, right)
	badRight.Payload = &types.LeafRevelation_LeafHash{LeafHash: make([]byte, types.LeafHashSize)}
	err = f.keeper.ApplyVerificationPayload(
		f.atHeight(504), f.validators[0].operator, 504, nil,
		[]types.CommitmentRevelation{
			{CommitmentHeight: 503, Left: left, Right: hashLeaf(t, right)},
			{CommitmentHeight: 503, Left: left, Right: badRight},
		},
	)
	require.ErrorIs(t, err, types.ErrDuplicateCommitmentRevelation)
	exists, err := f.keeper.VerificationVotes.Has(f.ctx, types.NewVerificationVoteStoreKey(500, 0, f.validators[0].operator))
	require.NoError(t, err)
	require.False(t, exists)
}

func TestFinalizationAndPruningPreserveHashLookup(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 500, 1)
	require.NoError(t, f.keeper.EndBlock(f.atHeight(500)))
	for validatorIndex := 0; validatorIndex < 2; validatorIndex++ {
		left := valueLeaf(t, byte(validatorIndex+1), []types.ProofVote{{IndexInBlock: 0, Result: true}})
		right := valueLeaf(t, byte(validatorIndex+10), nil)
		root := commitmentFor(t, left, right)
		err := f.keeper.ApplyVerificationPayload(f.atHeight(503), f.validators[validatorIndex].operator, 503, root, nil)
		require.NoError(t, err)
		err = f.keeper.ApplyVerificationPayload(
			f.atHeight(504), f.validators[validatorIndex].operator, 504, nil,
			[]types.CommitmentRevelation{{
				CommitmentHeight: 503, Left: left, Right: hashLeaf(t, right),
			}},
		)
		require.NoError(t, err)
	}

	require.NoError(t, f.keeper.EndBlock(f.atHeight(505)))
	result, err := f.keeper.FinalProofResults.Get(f.ctx, types.NewProofStoreKey(500, 0))
	require.NoError(t, err)
	require.Equal(t, types.ProofStatus_PROOF_STATUS_VALID, result.Status)
	require.Equal(t, int64(67), result.VotingPowerThreshold)

	pending, err := f.keeper.PendingProofs.Has(f.ctx, types.NewProofStoreKey(500, 0))
	require.NoError(t, err)
	require.False(t, pending)
	totalExists, err := f.keeper.TotalVotingPowerByHeight.Has(f.ctx, 500)
	require.NoError(t, err)
	require.False(t, totalExists)
	for _, validator := range f.validators {
		powerExists, err := f.keeper.ValidatorPowers.Has(f.ctx, types.NewValidatorPowerStoreKey(500, validator.operator))
		require.NoError(t, err)
		require.False(t, powerExists)
	}
	stored, err := f.keeper.FinalProofResults.Get(f.ctx, types.NewProofStoreKey(500, 0))
	require.NoError(t, err)
	response, err := f.query.ProofByVerificationId(f.ctx, &types.QueryProofByVerificationIdRequest{VerificationId: stored.VerificationId})
	require.NoError(t, err)
	require.NotNil(t, response.GetFinalResult())
}
