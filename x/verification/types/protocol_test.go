package types_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	comettypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func TestParamsValidate(t *testing.T) {
	for _, value := range []uint32{1, 128, 256} {
		require.NoError(t, types.NewParams(value).Validate())
	}
	for _, value := range []uint32{0, 257} {
		require.Error(t, types.NewParams(value).Validate())
	}
}

func TestMsgServiceExposesOnlyPublicTransactions(t *testing.T) {
	methods := make([]string, 0, len(types.Msg_serviceDesc.Methods))
	for _, method := range types.Msg_serviceDesc.Methods {
		methods = append(methods, method.MethodName)
	}
	require.Equal(t, []string{"SubmitProof", "UpdateParams"}, methods)
}

func TestCommitmentProofHeights(t *testing.T) {
	left, right, err := types.CommitmentProofHeights(503)
	require.NoError(t, err)
	require.Equal(t, uint64(500), left)
	require.Equal(t, uint64(501), right)
	_, _, err = types.CommitmentProofHeights(2)
	require.ErrorIs(t, err, types.ErrInvalidCommitmentHeight)
}

func TestEncodeVotes(t *testing.T) {
	votes := []types.ProofVote{{IndexInBlock: 0, Result: true}, {IndexInBlock: 255, Result: false}}
	encoded, err := types.EncodeVotes(votes)
	require.NoError(t, err)
	require.Equal(t, "00020001ff00", hex.EncodeToString(encoded))

	_, err = types.EncodeVotes([]types.ProofVote{{IndexInBlock: 1}, {IndexInBlock: 1}})
	require.ErrorIs(t, err, types.ErrDuplicateVoteIndex)
	_, err = types.EncodeVotes([]types.ProofVote{{IndexInBlock: 2}, {IndexInBlock: 1}})
	require.ErrorIs(t, err, types.ErrNonCanonicalVoteOrdering)
	_, err = types.EncodeVotes([]types.ProofVote{{IndexInBlock: 256}})
	require.ErrorIs(t, err, types.ErrInvalidVoteIndex)
}

func TestVotingPowerThresholdUsesStrictTwoThirdsMajority(t *testing.T) {
	expected := map[int64]int64{1: 1, 3: 3, 4: 3, 5: 4, 100: 67}
	for totalPower, threshold := range expected {
		actual, err := types.ComputeVotingPowerThreshold(totalPower)
		require.NoError(t, err)
		require.Equal(t, threshold, actual)
		require.False(t, types.HasTwoThirdsMajority(threshold-1, totalPower))
		require.True(t, types.HasTwoThirdsMajority(threshold, totalPower))
	}
	_, err := types.ComputeVotingPowerThreshold(0)
	require.ErrorIs(t, err, types.ErrEmptyValidatorSet)
	threshold, err := types.ComputeVotingPowerThreshold(comettypes.MaxTotalVotingPower)
	require.NoError(t, err)
	require.False(t, types.HasTwoThirdsMajority(threshold-1, comettypes.MaxTotalVotingPower))
	require.True(t, types.HasTwoThirdsMajority(threshold, comettypes.MaxTotalVotingPower))
	_, err = types.ComputeVotingPowerThreshold(comettypes.MaxTotalVotingPower + 1)
	require.ErrorIs(t, err, types.ErrProofStateCorrupted)
}

func TestGenesisValidateDerivesTalliesFromEffectiveVotes(t *testing.T) {
	state := validWeightedGenesisState()
	require.NoError(t, state.Validate())

	state.ProofTallies[0].Tally = types.ProofTally{}
	require.ErrorContains(t, state.Validate(), "proof tally does not match effective votes")
}

func TestGenesisValidateRejectsMismatchedVerificationID(t *testing.T) {
	state := validWeightedGenesisState()
	state.PendingProofs[0].Proof.VerificationId[0] ^= 0xff
	require.ErrorContains(t, state.Validate(), "invalid pending proof")
}

func TestGenesisValidateRejectsBrokenPowerRelationships(t *testing.T) {
	t.Run("power sum mismatch", func(t *testing.T) {
		state := validWeightedGenesisState()
		state.ValidatorPowers[0].VotingPower--
		require.ErrorContains(t, state.Validate(), "incomplete validator power snapshot")
	})
	t.Run("orphan total power", func(t *testing.T) {
		state := validWeightedGenesisState()
		state.TotalVotingPowers = append(state.TotalVotingPowers, types.GenesisTotalVotingPower{
			Height: 501, TotalVotingPower: 1,
		})
		require.ErrorContains(t, state.Validate(), "orphan total voting power")
	})
	t.Run("vote without historical power", func(t *testing.T) {
		state := validWeightedGenesisState()
		state.VerificationVotes[0].Validator = []byte{9}
		require.ErrorContains(t, state.Validate(), "not from an eligible validator")
	})
	t.Run("equivocation retains no contribution", func(t *testing.T) {
		state := validWeightedGenesisState()
		state.VerificationVotes[0].State = types.VoteState_VOTE_STATE_EQUIVOCATED
		require.ErrorContains(t, state.Validate(), "proof tally does not match effective votes")
	})
}

func TestGenesisValidateRecomputesFinalPowerResult(t *testing.T) {
	record := validProofRecord(t, 3)
	state := types.GenesisState{
		Params: types.DefaultParams(),
		SeenVerificationIds: []types.GenesisSeenVerificationId{{
			VerificationId: record.VerificationId, ProofKey: types.ProofKey{SubmissionHeight: 500},
		}},
		FinalProofResults: []types.GenesisFinalProofResult{{
			ProofKey: types.ProofKey{SubmissionHeight: 500},
			Result: types.FinalProofResult{
				ProofHash: record.ProofHash, ProofType: record.ProofType,
				PublicInputsHash: record.PublicInputsHash, VerificationKeyHash: record.VerificationKeyHash,
				VerificationId: record.VerificationId, Status: types.ProofStatus_PROOF_STATUS_VALID,
				ValidVotingPower: 67, TotalVotingPower: 100, VotingPowerThreshold: 67,
				SubmissionHeight: 500, FinalizedHeight: 505,
			},
		}},
	}
	require.NoError(t, state.Validate())

	state.FinalProofResults[0].Result.VotingPowerThreshold = 66
	require.ErrorContains(t, state.Validate(), "invalid final proof threshold")
	state.FinalProofResults[0].Result.VotingPowerThreshold = 67
	state.FinalProofResults[0].Result.Status = types.ProofStatus_PROOF_STATUS_INCONCLUSIVE
	require.ErrorContains(t, state.Validate(), "status does not match voting power")
}

func validWeightedGenesisState() types.GenesisState {
	record := validProofRecord(nil, 1)
	validator := []byte{2}
	return types.GenesisState{
		Params:      types.DefaultParams(),
		ProofCounts: []types.GenesisProofCount{{Height: 500, Count: 1}},
		PendingProofs: []types.GenesisPendingProof{{
			ProofKey: types.ProofKey{SubmissionHeight: 500},
			Proof:    record,
		}},
		SeenVerificationIds: []types.GenesisSeenVerificationId{{
			VerificationId: record.VerificationId, ProofKey: types.ProofKey{SubmissionHeight: 500},
		}},
		ValidatorPowers: []types.GenesisValidatorPower{{Height: 500, Validator: validator, VotingPower: 60}},
		TotalVotingPowers: []types.GenesisTotalVotingPower{{
			Height: 500, TotalVotingPower: 60,
		}},
		VerificationVotes: []types.GenesisVerificationVote{{
			ProofKey: types.ProofKey{SubmissionHeight: 500}, Validator: validator,
			State: types.VoteState_VOTE_STATE_TRUE,
		}},
		ProofTallies: []types.GenesisProofTally{{
			ProofKey: types.ProofKey{SubmissionHeight: 500},
			Tally:    types.ProofTally{ValidVotingPower: 60},
		}},
	}
}

func validProofRecord(t testing.TB, seed byte) types.ProofRecord {
	proofHash := bytes.Repeat([]byte{seed}, types.ProofHashSize)
	publicInputsHash := bytes.Repeat([]byte{seed + 1}, types.PublicInputsHashSize)
	verificationKeyHash := bytes.Repeat([]byte{seed + 2}, types.VerificationKeyHashSize)
	id, err := types.ComputeVerificationID(
		types.ProofType_PROOF_TYPE_MINA_PICKLES,
		proofHash,
		publicInputsHash,
		verificationKeyHash,
	)
	if t != nil {
		require.NoError(t, err)
	} else if err != nil {
		panic(err)
	}
	return types.ProofRecord{
		ProofHash:           proofHash,
		ProofType:           types.ProofType_PROOF_TYPE_MINA_PICKLES,
		PublicInputsHash:    publicInputsHash,
		VerificationKeyHash: verificationKeyHash,
		VerificationId:      id[:],
	}
}

func TestLeafTiming(t *testing.T) {
	require.Equal(t, types.LeafTooEarly, types.GetLeafTiming(503, 500))
	require.Equal(t, types.LeafActive, types.GetLeafTiming(504, 500))
	require.Equal(t, types.LeafActive, types.GetLeafTiming(505, 500))
	require.Equal(t, types.LeafExpired, types.GetLeafTiming(506, 500))
}

func TestOneofJSONRoundTrip(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	c := codec.NewProtoCodec(registry)

	responses := []*types.QueryProofResponse{
		{
			ProofKey: types.ProofKey{SubmissionHeight: 40},
			State:    &types.QueryProofResponse_Pending{Pending: ptrProofRecord(validProofRecord(t, 1))},
		},
		{
			ProofKey: types.ProofKey{SubmissionHeight: 40},
			State:    &types.QueryProofResponse_FinalResult{FinalResult: finalFromRecord(validProofRecord(t, 1))},
		},
	}
	for _, response := range responses {
		encoded, err := c.MarshalJSON(response)
		require.NoError(t, err)
		var decoded types.QueryProofResponse
		require.NoError(t, c.UnmarshalJSON(encoded, &decoded))
		require.Equal(t, response, &decoded)
	}

	leaf := &types.LeafRevelation{
		Mode: types.LeafRevealMode_LEAF_REVEAL_MODE_VALUE,
		Payload: &types.LeafRevelation_Value{Value: &types.RevealedLeaf{
			Salt:  bytes.Repeat([]byte{2}, types.SaltSize),
			Votes: []types.ProofVote{},
		}},
	}
	encoded, err := c.MarshalJSON(leaf)
	require.NoError(t, err)
	var decoded types.LeafRevelation
	require.NoError(t, c.UnmarshalJSON(encoded, &decoded))
	require.Equal(t, leaf, &decoded)
}

func ptrProofRecord(record types.ProofRecord) *types.ProofRecord { return &record }

func finalFromRecord(record types.ProofRecord) *types.FinalProofResult {
	return &types.FinalProofResult{
		ProofHash: record.ProofHash, ProofType: record.ProofType,
		PublicInputsHash: record.PublicInputsHash, VerificationKeyHash: record.VerificationKeyHash,
		VerificationId: record.VerificationId, Status: types.ProofStatus_PROOF_STATUS_INCONCLUSIVE,
	}
}

func FuzzCanonicalVoteEncoding(f *testing.F) {
	f.Add(uint32(0), true, uint32(255), false)
	f.Add(uint32(2), false, uint32(1), true)
	f.Fuzz(func(t *testing.T, first uint32, firstResult bool, second uint32, secondResult bool) {
		votes := []types.ProofVote{
			{IndexInBlock: first, Result: firstResult},
			{IndexInBlock: second, Result: secondResult},
		}
		encoded, err := types.EncodeVotes(votes)
		if err != nil {
			return
		}
		require.Len(t, encoded, 6)
		require.Equal(t, byte(first), encoded[2])
		require.Equal(t, byte(second), encoded[4])
	})
}

func BenchmarkCanonicalVoteEncoding(b *testing.B) {
	for _, count := range []int{0, 1, 64, 256} {
		votes := make([]types.ProofVote, count)
		for i := range votes {
			votes[i] = types.ProofVote{IndexInBlock: uint32(i), Result: i%2 == 0}
		}
		b.Run(fmt.Sprintf("%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_, err := types.EncodeVotes(votes)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkLeafHash(b *testing.B) {
	for _, count := range []int{0, 256} {
		votes := make([]types.ProofVote, count)
		for i := range votes {
			votes[i] = types.ProofVote{IndexInBlock: uint32(i), Result: i%2 == 0}
		}
		salt := make([]byte, types.SaltSize)
		b.Run(fmt.Sprintf("%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_, err := types.ComputeLeafHash(salt, votes)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
