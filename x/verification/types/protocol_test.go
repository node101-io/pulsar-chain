package types_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

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

func TestComputeThreshold(t *testing.T) {
	expected := map[uint32]uint32{3: 2, 4: 3, 5: 4, 6: 4, 10: 7}
	for validatorCount, threshold := range expected {
		actual, err := types.ComputeThreshold(validatorCount)
		require.NoError(t, err)
		require.Equal(t, threshold, actual)
	}
	_, err := types.ComputeThreshold(0)
	require.ErrorIs(t, err, types.ErrEmptyValidatorSet)
}

func TestGenesisValidateDerivesTalliesFromEffectiveVotes(t *testing.T) {
	hash := bytes.Repeat([]byte{1}, types.ProofHashSize)
	validator := []byte{2}
	state := types.GenesisState{
		Params:      types.DefaultParams(),
		ProofCounts: []types.GenesisProofCount{{Height: 500, Count: 1}},
		PendingProofs: []types.GenesisPendingProof{{
			ProofKey: types.ProofKey{SubmissionHeight: 500},
			Proof:    types.ProofRecord{ProofHash: hash},
		}},
		SeenProofHashes: []types.GenesisSeenProofHash{{
			ProofHash: hash, ProofKey: types.ProofKey{SubmissionHeight: 500},
		}},
		ValidatorSnapshots: []types.GenesisValidatorSnapshot{{Height: 500, Validator: validator}},
		ValidatorCounts:    []types.GenesisValidatorCount{{Height: 500, Count: 1}},
		VerificationVotes: []types.GenesisVerificationVote{{
			ProofKey: types.ProofKey{SubmissionHeight: 500}, Validator: validator,
			State: types.VoteState_VOTE_STATE_TRUE,
		}},
		ProofTallies: []types.GenesisProofTally{{
			ProofKey: types.ProofKey{SubmissionHeight: 500},
			Tally:    types.ProofTally{TrueVotes: 1},
		}},
	}
	require.NoError(t, state.Validate())

	state.ProofTallies[0].Tally = types.ProofTally{}
	require.ErrorContains(t, state.Validate(), "proof tally does not match effective votes")
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
			State: &types.QueryProofResponse_Pending{Pending: &types.ProofRecord{
				ProofHash: bytes.Repeat([]byte{1}, types.ProofHashSize),
			}},
		},
		{
			ProofKey: types.ProofKey{SubmissionHeight: 40},
			State: &types.QueryProofResponse_FinalResult{FinalResult: &types.FinalProofResult{
				ProofHash: bytes.Repeat([]byte{1}, types.ProofHashSize),
				Status:    types.ProofStatus_PROOF_STATUS_INCONCLUSIVE,
			}},
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
