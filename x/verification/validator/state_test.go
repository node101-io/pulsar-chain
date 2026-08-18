package validator

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

func TestStateValidationRejectsCorruptedSecrets(t *testing.T) {
	valid := validTestState(t)
	tests := []struct {
		name   string
		mutate func(*State)
	}{
		{name: "version", mutate: func(s *State) { s.Version++ }},
		{name: "partial binding", mutate: func(s *State) { s.ChainID = "" }},
		{name: "key fingerprint", mutate: func(s *State) { s.ValidatorConsensusKeyHash = []byte{1} }},
		{name: "wrong left height", mutate: func(s *State) { s.Commitments[0].Left.ProofHeight++ }},
		{name: "short salt", mutate: func(s *State) { s.Commitments[0].Left.Salt = []byte{1} }},
		{name: "non canonical votes", mutate: func(s *State) {
			s.Commitments[0].Left.Votes = []verificationtypes.ProofVote{{IndexInBlock: 2}, {IndexInBlock: 1}}
		}},
		{name: "leaf mismatch", mutate: func(s *State) { s.Commitments[0].Left.LeafHash[0] ^= 1 }},
		{name: "short root", mutate: func(s *State) { s.Commitments[0].CommitmentRoot = []byte{1} }},
		{name: "root mismatch", mutate: func(s *State) { s.Commitments[0].CommitmentRoot[0] ^= 1 }},
		{name: "duplicate height", mutate: func(s *State) { s.Commitments = append(s.Commitments, s.Commitments[0]) }},
		{name: "too many records", mutate: func(s *State) {
			for len(s.Commitments) <= verificationtypes.MaxRevelationsPerBatch+1 {
				next := s.Commitments[len(s.Commitments)-1].clone()
				next.CommitmentHeight++
				next.Left.ProofHeight++
				next.Right.ProofHeight++
				s.Commitments = append(s.Commitments, next)
			}
		}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			state := valid.Clone()
			testCase.mutate(&state)
			require.ErrorIs(t, ValidateState(state, false), ErrInvalidLocalState)
		})
	}
}

func TestStateCollectionAndMemoryStoreErrors(t *testing.T) {
	state := validTestState(t)
	require.ErrorIs(t, state.PutCommitment(state.Commitments[0]), ErrInvalidLocalState)
	var nilState *State
	require.ErrorIs(t, nilState.PutCommitment(CommitmentSecret{}), ErrInvalidLocalState)

	store := NewMemoryStore()
	store.SetLoadError(errors.New("load failed"))
	_, err := store.Load()
	require.ErrorContains(t, err, "load failed")
	_, err = NewBuilder(newReaderStub(), nil, store, nil, 1)
	require.ErrorContains(t, err, "load failed")
}

func TestDecodeStateRejectsTrailingAndInvalidHex(t *testing.T) {
	encoded, err := encodeState(validTestState(t))
	require.NoError(t, err)
	_, err = decodeState(append(encoded, []byte(` {}`)...))
	require.ErrorIs(t, err, ErrInvalidLocalState)

	invalidHex := bytes.Replace(encoded, []byte(`"validator_operator_address": "`), []byte(`"validator_operator_address": "zz`), 1)
	_, err = decodeState(invalidHex)
	require.ErrorIs(t, err, ErrInvalidLocalState)
}

func FuzzDecodeStateNeverPanics(f *testing.F) {
	state := validTestState(f)
	encoded, err := encodeState(state)
	if err == nil {
		f.Add(encoded)
	}
	f.Add([]byte(`{"version":1}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = decodeState(data)
	})
}
