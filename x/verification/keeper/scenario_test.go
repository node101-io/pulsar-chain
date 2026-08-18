package keeper_test

import (
	"bytes"
	"testing"

	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func submitCommitment(
	t testing.TB,
	f *fixture,
	validatorIndex int,
	height int64,
	left,
	right types.LeafRevelation,
) {
	t.Helper()
	err := f.keeper.ApplyVerificationPayload(
		f.atHeight(height), f.validators[validatorIndex].operator, uint64(height),
		commitmentFor(t, left, right), nil,
	)
	require.NoError(t, err)
}

func reveal(
	t testing.TB,
	f *fixture,
	validatorIndex int,
	height int64,
	revelations ...types.CommitmentRevelation,
) error {
	t.Helper()
	return f.keeper.ApplyVerificationPayload(
		f.atHeight(height), f.validators[validatorIndex].operator, uint64(height), nil, revelations,
	)
}

func TestProofCapacityAndDeterministicIndices(t *testing.T) {
	f := initFixture(t, 3)
	require.NoError(t, f.keeper.Params.Set(f.ctx, types.Params{MaxProofsPerBlock: 2}))

	require.Equal(t, uint32(0), submitProof(t, f, 500, 1).IndexInBlock)
	require.Equal(t, uint32(1), submitProof(t, f, 500, 2).IndexInBlock)
	hash := bytes.Repeat([]byte{3}, types.ProofHashSize)
	_, err := f.msgServer.SubmitProof(f.atHeight(500), &types.MsgSubmitProof{ProofHash: hash})
	require.ErrorIs(t, err, types.ErrMaxProofsPerBlock)

	seen, err := f.keeper.SeenProofHashes.Has(f.ctx, hash)
	require.NoError(t, err)
	require.False(t, seen)
}

func TestUpdateParamsRequiresAuthorityAndValidBounds(t *testing.T) {
	f := initFixture(t, 3)
	_, err := f.msgServer.UpdateParams(f.ctx, &types.MsgUpdateParams{
		Authority: f.validators[0].signer, Params: types.NewParams(2),
	})
	require.ErrorIs(t, err, types.ErrInvalidSigner)
	authority, err := f.accountCodec.BytesToString(authtypes.NewModuleAddress(types.GovModuleName))
	require.NoError(t, err)
	_, err = f.msgServer.UpdateParams(f.ctx, &types.MsgUpdateParams{
		Authority: authority, Params: types.NewParams(0),
	})
	require.Error(t, err)
	_, err = f.msgServer.UpdateParams(f.ctx, &types.MsgUpdateParams{
		Authority: authority, Params: types.NewParams(2),
	})
	require.NoError(t, err)
	params, err := f.keeper.Params.Get(f.ctx)
	require.NoError(t, err)
	require.Equal(t, uint32(2), params.MaxProofsPerBlock)
}

func TestValidatorSnapshotIsStableWithinProofHeight(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 500, 1)
	f.staking.lastValidators = f.staking.lastValidators[:2]
	submitProof(t, f, 500, 2)

	count, err := f.keeper.ValidatorCountByHeight.Get(f.ctx, 500)
	require.NoError(t, err)
	require.Equal(t, uint32(3), count)

	submitProof(t, f, 501, 3)
	count, err = f.keeper.ValidatorCountByHeight.Get(f.ctx, 501)
	require.NoError(t, err)
	require.Equal(t, uint32(2), count)
}

func TestCommitmentWindowsAcceptDifferentProofSubsets(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 500, 1)
	submitProof(t, f, 500, 2)

	left502 := valueLeaf(t, 1, nil)
	right502 := valueLeaf(t, 2, []types.ProofVote{{IndexInBlock: 0, Result: true}})
	submitCommitment(t, f, 0, 502, left502, right502)

	left503 := valueLeaf(t, 3, []types.ProofVote{{IndexInBlock: 1, Result: false}})
	right503 := valueLeaf(t, 4, nil)
	submitCommitment(t, f, 0, 503, left503, right503)

	require.NoError(t, reveal(t, f, 0, 504,
		types.CommitmentRevelation{CommitmentHeight: 502, Left: hashLeaf(t, left502), Right: right502},
		types.CommitmentRevelation{CommitmentHeight: 503, Left: left503, Right: hashLeaf(t, right503)},
	))

	vote0, err := f.keeper.VerificationVotes.Get(f.ctx, types.NewVerificationVoteStoreKey(500, 0, f.validators[0].operator))
	require.NoError(t, err)
	vote1, err := f.keeper.VerificationVotes.Get(f.ctx, types.NewVerificationVoteStoreKey(500, 1, f.validators[0].operator))
	require.NoError(t, err)
	require.Equal(t, uint32(types.VoteState_VOTE_STATE_TRUE), vote0)
	require.Equal(t, uint32(types.VoteState_VOTE_STATE_FALSE), vote1)
}

func TestDuplicateVoteIsIdempotent(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 500, 1)

	left502 := valueLeaf(t, 1, nil)
	right502 := valueLeaf(t, 2, []types.ProofVote{{IndexInBlock: 0, Result: true}})
	submitCommitment(t, f, 0, 502, left502, right502)
	left503 := valueLeaf(t, 3, []types.ProofVote{{IndexInBlock: 0, Result: true}})
	right503 := valueLeaf(t, 4, nil)
	submitCommitment(t, f, 0, 503, left503, right503)

	require.NoError(t, reveal(t, f, 0, 504,
		types.CommitmentRevelation{CommitmentHeight: 502, Left: hashLeaf(t, left502), Right: right502},
		types.CommitmentRevelation{CommitmentHeight: 503, Left: left503, Right: hashLeaf(t, right503)},
	))
	tally, err := f.keeper.ProofTallies.Get(f.ctx, types.NewProofStoreKey(500, 0))
	require.NoError(t, err)
	require.Equal(t, uint32(1), tally.TrueVotes)
	require.Zero(t, tally.FalseVotes)
}

func TestExpiredValueReconstructsButDoesNotVote(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 499, 1)
	submitProof(t, f, 500, 2)
	left := valueLeaf(t, 1, []types.ProofVote{{IndexInBlock: 0, Result: true}})
	right := valueLeaf(t, 2, []types.ProofVote{{IndexInBlock: 0, Result: false}})
	submitCommitment(t, f, 0, 502, left, right)

	require.NoError(t, reveal(t, f, 0, 505, types.CommitmentRevelation{
		CommitmentHeight: 502, Left: left, Right: right,
	}))
	tally499, err := f.keeper.ProofTallies.Get(f.ctx, types.NewProofStoreKey(499, 0))
	require.NoError(t, err)
	tally500, err := f.keeper.ProofTallies.Get(f.ctx, types.NewProofStoreKey(500, 0))
	require.NoError(t, err)
	require.Equal(t, types.ProofTally{}, tally499)
	require.Equal(t, uint32(1), tally500.FalseVotes)
}

func TestEarlyRevealAndUselessRevealAreRejected(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 500, 1)
	left := valueLeaf(t, 1, []types.ProofVote{{IndexInBlock: 0, Result: true}})
	right := valueLeaf(t, 2, nil)
	submitCommitment(t, f, 0, 503, left, right)

	err := reveal(t, f, 0, 504, types.CommitmentRevelation{CommitmentHeight: 503, Left: left, Right: right})
	require.ErrorIs(t, err, types.ErrEarlyReveal)
	err = reveal(t, f, 0, 504, types.CommitmentRevelation{
		CommitmentHeight: 503, Left: hashLeaf(t, left), Right: hashLeaf(t, right),
	})
	require.ErrorIs(t, err, types.ErrUselessRevelation)
}

func TestPublicMessagesRejectMalformedLengthsWithoutWrites(t *testing.T) {
	f := initFixture(t, 3)
	_, err := f.msgServer.SubmitProof(f.atHeight(500), &types.MsgSubmitProof{
		ProofHash: make([]byte, types.ProofHashSize-1),
	})
	require.ErrorIs(t, err, types.ErrInvalidProofHash)
	exists, err := f.keeper.ProofCountByHeight.Has(f.ctx, 500)
	require.NoError(t, err)
	require.False(t, exists)

	err = f.keeper.ApplyVerificationPayload(
		f.atHeight(503), f.validators[0].operator, 503,
		make([]byte, types.CommitmentHashSize-1), nil,
	)
	require.ErrorIs(t, err, types.ErrInvalidCommitmentLength)
	exists, err = f.keeper.Commitments.Has(f.ctx, types.NewCommitmentStoreKey(f.validators[0].operator, 503))
	require.NoError(t, err)
	require.False(t, exists)
}

func TestRevealRejectsMalformedLeavesAndVotesWithoutWrites(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 500, 1)
	submitProof(t, f, 500, 2)

	left := valueLeaf(t, 1, []types.ProofVote{{IndexInBlock: 0, Result: true}})
	right := valueLeaf(t, 2, nil)
	submitCommitment(t, f, 0, 503, left, right)

	revealFor := func(left, right types.LeafRevelation) error {
		return reveal(t, f, 0, 504, types.CommitmentRevelation{
			CommitmentHeight: 503, Left: left, Right: right,
		})
	}

	badSalt := left
	badSalt.Payload = &types.LeafRevelation_Value{Value: &types.RevealedLeaf{
		Salt: make([]byte, types.SaltSize-1), Votes: left.GetValue().Votes,
	}}
	require.ErrorIs(t, revealFor(badSalt, hashLeaf(t, right)), types.ErrInvalidSaltLength)

	badHash := hashLeaf(t, right)
	badHash.Payload = &types.LeafRevelation_LeafHash{LeafHash: make([]byte, types.LeafHashSize-1)}
	require.ErrorIs(t, revealFor(left, badHash), types.ErrInvalidLeafHashLength)

	badMode := left
	badMode.Mode = types.LeafRevealMode_LEAF_REVEAL_MODE_UNSPECIFIED
	badMode.Payload = nil
	require.ErrorIs(t, revealFor(badMode, hashLeaf(t, right)), types.ErrInvalidLeafRevealMode)

	duplicate := valueLeaf(t, 1, []types.ProofVote{
		{IndexInBlock: 0, Result: true},
		{IndexInBlock: 0, Result: false},
	})
	require.ErrorIs(t, revealFor(duplicate, hashLeaf(t, right)), types.ErrDuplicateVoteIndex)

	nonCanonical := valueLeaf(t, 1, []types.ProofVote{
		{IndexInBlock: 1, Result: true},
		{IndexInBlock: 0, Result: false},
	})
	require.ErrorIs(t, revealFor(nonCanonical, hashLeaf(t, right)), types.ErrNonCanonicalVoteOrdering)

	nonexistent := valueLeaf(t, 3, []types.ProofVote{{IndexInBlock: 2, Result: true}})
	validatorOneRight := valueLeaf(t, 4, nil)
	submitCommitment(t, f, 1, 503, nonexistent, validatorOneRight)
	require.ErrorIs(t, reveal(t, f, 1, 504, types.CommitmentRevelation{
		CommitmentHeight: 503, Left: nonexistent, Right: hashLeaf(t, validatorOneRight),
	}), types.ErrProofNotFound)

	require.ErrorIs(t, reveal(t, f, 0, 504), types.ErrInvalidRevelationCount)
	require.ErrorIs(t, reveal(t, f, 0, 504,
		types.CommitmentRevelation{},
		types.CommitmentRevelation{},
		types.CommitmentRevelation{},
		types.CommitmentRevelation{},
	), types.ErrInvalidRevelationCount)

	for _, validator := range f.validators[:2] {
		for index := uint32(0); index < 2; index++ {
			exists, err := f.keeper.VerificationVotes.Has(f.ctx, types.NewVerificationVoteStoreKey(500, index, validator.operator))
			require.NoError(t, err)
			require.False(t, exists)
		}
	}
}

func TestDistinctInvalidLaterRevelationIsAtomic(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 500, 1)

	left502 := valueLeaf(t, 1, nil)
	right502 := valueLeaf(t, 2, []types.ProofVote{{IndexInBlock: 0, Result: true}})
	submitCommitment(t, f, 0, 502, left502, right502)
	left503 := valueLeaf(t, 3, []types.ProofVote{{IndexInBlock: 0, Result: false}})
	right503 := valueLeaf(t, 4, nil)
	submitCommitment(t, f, 0, 503, left503, right503)

	badRight := hashLeaf(t, right503)
	badRight.Payload = &types.LeafRevelation_LeafHash{LeafHash: bytes.Repeat([]byte{9}, types.LeafHashSize)}
	err := reveal(t, f, 0, 504,
		types.CommitmentRevelation{CommitmentHeight: 502, Left: hashLeaf(t, left502), Right: right502},
		types.CommitmentRevelation{CommitmentHeight: 503, Left: left503, Right: badRight},
	)
	require.ErrorIs(t, err, types.ErrCommitmentMismatch)

	exists, err := f.keeper.VerificationVotes.Has(f.ctx, types.NewVerificationVoteStoreKey(500, 0, f.validators[0].operator))
	require.NoError(t, err)
	require.False(t, exists)
}

func TestFinalizationProducesInvalidAndInconclusiveResults(t *testing.T) {
	for _, testCase := range []struct {
		name           string
		validatorVotes int
		result         bool
		expected       types.ProofStatus
	}{
		{name: "invalid", validatorVotes: 2, result: false, expected: types.ProofStatus_PROOF_STATUS_INVALID},
		{name: "inconclusive", validatorVotes: 1, result: true, expected: types.ProofStatus_PROOF_STATUS_INCONCLUSIVE},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			f := initFixture(t, 3)
			submitProof(t, f, 500, 1)
			for validatorIndex := 0; validatorIndex < testCase.validatorVotes; validatorIndex++ {
				left := valueLeaf(t, byte(validatorIndex+1), []types.ProofVote{{IndexInBlock: 0, Result: testCase.result}})
				right := valueLeaf(t, byte(validatorIndex+10), nil)
				submitCommitment(t, f, validatorIndex, 503, left, right)
				require.NoError(t, reveal(t, f, validatorIndex, 504, types.CommitmentRevelation{
					CommitmentHeight: 503, Left: left, Right: hashLeaf(t, right),
				}))
			}

			require.NoError(t, f.keeper.EndBlock(f.atHeight(505)))
			result, err := f.keeper.FinalProofResults.Get(f.ctx, types.NewProofStoreKey(500, 0))
			require.NoError(t, err)
			require.Equal(t, testCase.expected, result.Status)
		})
	}
}

func TestCorruptedFinalizationLeavesLifecycleStateUnchanged(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 500, 1)
	submitProof(t, f, 500, 2)
	require.NoError(t, f.keeper.ProofTallies.Set(f.ctx, types.NewProofStoreKey(500, 1), types.ProofTally{TrueVotes: 4}))

	err := f.keeper.EndBlock(f.atHeight(505))
	require.ErrorIs(t, err, types.ErrProofStateCorrupted)
	finalized, err := f.keeper.FinalProofResults.Has(f.ctx, types.NewProofStoreKey(500, 0))
	require.NoError(t, err)
	require.False(t, finalized)
	pending, err := f.keeper.PendingProofs.Has(f.ctx, types.NewProofStoreKey(500, 0))
	require.NoError(t, err)
	require.True(t, pending)
}

func TestQueriesPaginateAndCommitmentPruningUsesReverseIndex(t *testing.T) {
	f := initFixture(t, 3)
	for i := byte(1); i <= 3; i++ {
		submitProof(t, f, 500, i)
	}
	pageOne, err := f.query.ProofsByHeight(f.ctx, &types.QueryProofsByHeightRequest{
		SubmissionHeight: 500, Pagination: &query.PageRequest{Limit: 2},
	})
	require.NoError(t, err)
	require.Len(t, pageOne.Proofs, 2)
	require.NotEmpty(t, pageOne.Pagination.NextKey)
	pageTwo, err := f.query.ProofsByHeight(f.ctx, &types.QueryProofsByHeightRequest{
		SubmissionHeight: 500, Pagination: &query.PageRequest{Key: pageOne.Pagination.NextKey, Limit: 2},
	})
	require.NoError(t, err)
	require.Len(t, pageTwo.Proofs, 1)

	left := valueLeaf(t, 1, nil)
	right := valueLeaf(t, 2, nil)
	submitCommitment(t, f, 0, 502, left, right)
	require.NoError(t, f.keeper.PruneCommitmentsAtHeight(f.ctx, 502))
	primary, err := f.keeper.Commitments.Has(f.ctx, types.NewCommitmentStoreKey(f.validators[0].operator, 502))
	require.NoError(t, err)
	require.False(t, primary)
	reverse, err := f.keeper.CommitmentsByHeight.Has(f.ctx, types.NewCommitmentHeightStoreKey(502, f.validators[0].operator))
	require.NoError(t, err)
	require.False(t, reverse)
}

func TestQuerySurfaceReturnsCommitmentsVotesTalliesAndResults(t *testing.T) {
	f := initFixture(t, 3)
	submitProof(t, f, 500, 1)
	left := valueLeaf(t, 1, []types.ProofVote{{IndexInBlock: 0, Result: true}})
	right := valueLeaf(t, 2, nil)
	submitCommitment(t, f, 0, 503, left, right)

	commitment, err := f.query.Commitment(f.ctx, &types.QueryCommitmentRequest{
		Validator: f.validators[0].operatorS, CommitmentHeight: 503,
	})
	require.NoError(t, err)
	require.Equal(t, commitmentFor(t, left, right), commitment.Commitment.Commitment.Commitment)
	commitments, err := f.query.CommitmentsByValidator(f.ctx, &types.QueryCommitmentsByValidatorRequest{
		Validator: f.validators[0].operatorS,
	})
	require.NoError(t, err)
	require.Len(t, commitments.Commitments, 1)

	require.NoError(t, reveal(t, f, 0, 504, types.CommitmentRevelation{
		CommitmentHeight: 503, Left: left, Right: hashLeaf(t, right),
	}))
	vote, err := f.query.Vote(f.ctx, &types.QueryVoteRequest{
		SubmissionHeight: 500, IndexInBlock: 0, Validator: f.validators[0].operatorS,
	})
	require.NoError(t, err)
	require.Equal(t, types.VoteState_VOTE_STATE_TRUE, vote.Vote.State)
	votes, err := f.query.VerificationVotes(f.ctx, &types.QueryVerificationVotesRequest{
		SubmissionHeight: 500, IndexInBlock: 0,
	})
	require.NoError(t, err)
	require.Len(t, votes.Votes, 1)
	tally, err := f.query.ProofTally(f.ctx, &types.QueryProofTallyRequest{SubmissionHeight: 500, IndexInBlock: 0})
	require.NoError(t, err)
	require.Equal(t, uint32(1), tally.Tally.TrueVotes)

	require.NoError(t, f.keeper.EndBlock(f.atHeight(505)))
	final, err := f.query.FinalProofResult(f.ctx, &types.QueryFinalProofResultRequest{SubmissionHeight: 500, IndexInBlock: 0})
	require.NoError(t, err)
	require.Equal(t, types.ProofStatus_PROOF_STATUS_INCONCLUSIVE, final.FinalResult.Status)
	_, err = f.query.ProofTally(f.ctx, &types.QueryProofTallyRequest{SubmissionHeight: 500, IndexInBlock: 0})
	require.ErrorIs(t, err, types.ErrProofNotFound)
}

func TestQueriesRejectCorruptedProofRelationships(t *testing.T) {
	fixture := initFixture(t, 3)
	submitProof(t, fixture, 500, 1)
	key := types.NewProofStoreKey(500, 0)
	record, err := fixture.keeper.PendingProofs.Get(fixture.ctx, key)
	require.NoError(t, err)

	require.NoError(t, fixture.keeper.FinalProofResults.Set(fixture.ctx, key, types.FinalProofResult{
		ProofHash: record.ProofHash, Status: types.ProofStatus_PROOF_STATUS_INCONCLUSIVE,
		EligibleValidatorCount: 3, Threshold: 2, SubmissionHeight: 500, FinalizedHeight: 505,
	}))
	_, err = fixture.query.Proof(fixture.ctx, &types.QueryProofRequest{SubmissionHeight: 500})
	require.ErrorIs(t, err, types.ErrProofStateCorrupted)
	require.NoError(t, fixture.keeper.FinalProofResults.Remove(fixture.ctx, key))

	require.NoError(t, fixture.keeper.SeenProofHashes.Set(fixture.ctx, record.ProofHash, types.ProofKey{
		SubmissionHeight: 500, IndexInBlock: 1,
	}))
	_, err = fixture.query.Proof(fixture.ctx, &types.QueryProofRequest{SubmissionHeight: 500})
	require.ErrorIs(t, err, types.ErrProofStateCorrupted)
	_, err = fixture.query.ProofByHash(fixture.ctx, &types.QueryProofByHashRequest{ProofHash: record.ProofHash})
	require.ErrorIs(t, err, types.ErrProofStateCorrupted)

	require.NoError(t, fixture.keeper.SeenProofHashes.Set(fixture.ctx, record.ProofHash, types.ProofKey{SubmissionHeight: 500}))
	require.NoError(t, fixture.keeper.EndBlock(fixture.atHeight(505)))
	result, err := fixture.keeper.FinalProofResults.Get(fixture.ctx, key)
	require.NoError(t, err)
	result.Status = types.ProofStatus_PROOF_STATUS_VALID
	require.NoError(t, fixture.keeper.FinalProofResults.Set(fixture.ctx, key, result))
	_, err = fixture.query.FinalProofResult(fixture.ctx, &types.QueryFinalProofResultRequest{SubmissionHeight: 500})
	require.ErrorIs(t, err, types.ErrProofStateCorrupted)
}

func FuzzVoteTransitionTallyConsistency(f *testing.F) {
	f.Add(uint8(0b111), uint8(0b111), uint8(0b101), uint8(0b101))
	f.Add(uint8(0b111), uint8(0b111), uint8(0b101), uint8(0b010))
	f.Add(uint8(0), uint8(0b101), uint8(0), uint8(0b001))

	f.Fuzz(func(t *testing.T, firstMask, secondMask, firstResults, secondResults uint8) {
		const validatorMask = uint8(0b111)
		firstMask &= validatorMask
		secondMask &= validatorMask
		firstResults &= validatorMask
		secondResults &= validatorMask

		fixture := initFixture(t, 3)
		submitProof(t, fixture, 500, 1)

		var expectedTrue, expectedFalse uint32
		for validatorIndex := range 3 {
			bit := uint8(1 << validatorIndex)
			firstVotes := fuzzVotes(firstMask&bit != 0, firstResults&bit != 0)
			secondVotes := fuzzVotes(secondMask&bit != 0, secondResults&bit != 0)

			left502 := valueLeaf(t, byte(10+validatorIndex), nil)
			right502 := valueLeaf(t, byte(20+validatorIndex), firstVotes)
			left503 := valueLeaf(t, byte(30+validatorIndex), secondVotes)
			right503 := valueLeaf(t, byte(40+validatorIndex), nil)
			submitCommitment(t, fixture, validatorIndex, 502, left502, right502)
			submitCommitment(t, fixture, validatorIndex, 503, left503, right503)

			require.NoError(t, reveal(t, fixture, validatorIndex, 504,
				types.CommitmentRevelation{CommitmentHeight: 502, Left: hashLeaf(t, left502), Right: right502},
				types.CommitmentRevelation{CommitmentHeight: 503, Left: left503, Right: hashLeaf(t, right503)},
			))

			expected := expectedVoteState(firstMask&bit != 0, firstResults&bit != 0, secondMask&bit != 0, secondResults&bit != 0)
			key := types.NewVerificationVoteStoreKey(500, 0, fixture.validators[validatorIndex].operator)
			exists, err := fixture.keeper.VerificationVotes.Has(fixture.ctx, key)
			require.NoError(t, err)
			if expected == types.VoteState_VOTE_STATE_NONE {
				require.False(t, exists)
				continue
			}
			require.True(t, exists)
			stored, err := fixture.keeper.VerificationVotes.Get(fixture.ctx, key)
			require.NoError(t, err)
			require.Equal(t, uint32(expected), stored)
			switch expected {
			case types.VoteState_VOTE_STATE_TRUE:
				expectedTrue++
			case types.VoteState_VOTE_STATE_FALSE:
				expectedFalse++
			}
		}

		tally, err := fixture.keeper.ProofTallies.Get(fixture.ctx, types.NewProofStoreKey(500, 0))
		require.NoError(t, err)
		require.Equal(t, expectedTrue, tally.TrueVotes)
		require.Equal(t, expectedFalse, tally.FalseVotes)
		require.LessOrEqual(t, tally.TrueVotes+tally.FalseVotes, uint32(3))
	})
}

func fuzzVotes(enabled, result bool) []types.ProofVote {
	if !enabled {
		return nil
	}
	return []types.ProofVote{{IndexInBlock: 0, Result: result}}
}

func expectedVoteState(firstEnabled, firstResult, secondEnabled, secondResult bool) types.VoteState {
	if !firstEnabled && !secondEnabled {
		return types.VoteState_VOTE_STATE_NONE
	}
	if firstEnabled && secondEnabled && firstResult != secondResult {
		return types.VoteState_VOTE_STATE_EQUIVOCATED
	}
	result := firstResult
	if secondEnabled {
		result = secondResult
	}
	if result {
		return types.VoteState_VOTE_STATE_TRUE
	}
	return types.VoteState_VOTE_STATE_FALSE
}
