package validator

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/verification/sidecar"
	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

func TestBuildResultRequestValidatesConsensusProofs(t *testing.T) {
	left := proofEntry(7, 0, 1)
	right := proofEntry(8, 1, 2)
	request, index, err := buildResultRequest(7, 8, []verificationtypes.ProofEntry{left}, []verificationtypes.ProofEntry{right}, nil)
	require.NoError(t, err)
	require.Equal(t, [][]byte{left.Record.VerificationId, right.Record.VerificationId}, request)
	require.Len(t, index, 2)

	for _, testCase := range []struct {
		name  string
		left  []verificationtypes.ProofEntry
		right []verificationtypes.ProofEntry
	}{
		{name: "wrong height", left: []verificationtypes.ProofEntry{proofEntry(6, 0, 1)}},
		{name: "invalid index", left: []verificationtypes.ProofEntry{proofEntry(7, 256, 1)}},
		{name: "invalid descriptor", left: []verificationtypes.ProofEntry{{Key: left.Key, Record: verificationtypes.ProofRecord{ProofHash: []byte{1}}}}},
		{name: "duplicate ID", left: []verificationtypes.ProofEntry{left}, right: []verificationtypes.ProofEntry{{Key: right.Key, Record: left.Record}}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, err := buildResultRequest(7, 8, testCase.left, testCase.right, nil)
			require.ErrorIs(t, err, verificationtypes.ErrProofStateCorrupted)
		})
	}

	tooMany := make([]verificationtypes.ProofEntry, verificationtypes.MaxVoteIndexExclusive*2+1)
	_, _, err = buildResultRequest(7, 8, tooMany, nil, nil)
	require.ErrorIs(t, err, verificationtypes.ErrTooManyVotes)
}

func TestValidateResultsRejectsMalformedWholeResponse(t *testing.T) {
	proof := proofEntry(7, 0, 1)
	requested := map[string]requestedProof{string(proof.Record.VerificationId): {height: 7, index: 0}}
	valid := sidecar.VerificationResult{VerificationID: proof.Record.VerificationId, Result: sidecar.ResultValid}

	left, right, err := validateResults(requested, []sidecar.VerificationResult{valid}, 7)
	require.NoError(t, err)
	require.Equal(t, []verificationtypes.ProofVote{{IndexInBlock: 0, Result: true}}, left)
	require.Empty(t, right)

	for _, testCase := range []struct {
		name    string
		results []sidecar.VerificationResult
	}{
		{name: "too many", results: []sidecar.VerificationResult{valid, valid}},
		{name: "short ID", results: []sidecar.VerificationResult{{VerificationID: []byte{1}, Result: sidecar.ResultValid}}},
		{name: "unknown ID", results: []sidecar.VerificationResult{{VerificationID: bytes.Repeat([]byte{9}, verificationtypes.VerificationIDSize), Result: sidecar.ResultValid}}},
		{name: "duplicate", results: []sidecar.VerificationResult{valid, valid}},
		{name: "unspecified", results: []sidecar.VerificationResult{{VerificationID: proof.Record.VerificationId, Result: sidecar.ResultUnspecified}}},
		{name: "unknown enum", results: []sidecar.VerificationResult{{VerificationID: proof.Record.VerificationId, Result: sidecar.ResultValue(99)}}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, err := validateResults(requested, testCase.results, 7)
			require.Error(t, err)
		})
	}
}

func FuzzValidateResultsNeverPanics(f *testing.F) {
	f.Add(bytes.Repeat([]byte{1}, verificationtypes.VerificationIDSize), uint8(sidecar.ResultValid))
	f.Add([]byte{1}, uint8(255))
	requestedID := bytes.Repeat([]byte{1}, verificationtypes.VerificationIDSize)
	requested := map[string]requestedProof{string(requestedID): {height: 7, index: 0}}
	f.Fuzz(func(t *testing.T, hash []byte, value uint8) {
		_, _, _ = validateResults(requested, []sidecar.VerificationResult{{VerificationID: hash, Result: sidecar.ResultValue(value)}}, 7)
	})
}

func proofEntry(height uint64, index uint32, seed byte) verificationtypes.ProofEntry {
	return verificationtypes.ProofEntry{
		Key:    verificationtypes.ProofKey{SubmissionHeight: height, IndexInBlock: index},
		Record: testProofRecord(seed),
	}
}
