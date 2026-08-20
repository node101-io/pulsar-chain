package types_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func TestComputeVerificationIDCrossLanguageVector(t *testing.T) {
	proofHash := bytes.Repeat([]byte{1}, types.ProofHashSize)
	publicInputsHash := bytes.Repeat([]byte{2}, types.PublicInputsHashSize)
	verificationKeyHash := bytes.Repeat([]byte{3}, types.VerificationKeyHashSize)

	id, err := types.ComputeVerificationID(
		types.ProofType_PROOF_TYPE_MINA_PICKLES,
		proofHash,
		publicInputsHash,
		verificationKeyHash,
	)
	require.NoError(t, err)
	require.Equal(t, "d2548149a7d662657a2a4b4a15b18f6f49fd85c114f6958c4db423edb4bd3e35", hex.EncodeToString(id[:]))

	differentType, err := types.ComputeVerificationID(
		types.ProofType_PROOF_TYPE_NOIR_BARRETENBERG,
		proofHash,
		publicInputsHash,
		verificationKeyHash,
	)
	require.NoError(t, err)
	require.NotEqual(t, id, differentType)

	differentOrder, err := types.ComputeVerificationID(
		types.ProofType_PROOF_TYPE_MINA_PICKLES,
		proofHash,
		verificationKeyHash,
		publicInputsHash,
	)
	require.NoError(t, err)
	require.NotEqual(t, id, differentOrder)
}

func TestComputeVerificationIDRejectsMalformedDescriptor(t *testing.T) {
	valid := bytes.Repeat([]byte{1}, types.DigestSize)
	testCases := []struct {
		name             string
		proofType        types.ProofType
		proofHash        []byte
		publicInputsHash []byte
		keyHash          []byte
		expected         error
	}{
		{name: "unspecified type", proofType: types.ProofType_PROOF_TYPE_UNSPECIFIED, proofHash: valid, publicInputsHash: valid, keyHash: valid, expected: types.ErrInvalidProofType},
		{name: "unknown type", proofType: types.ProofType(99), proofHash: valid, publicInputsHash: valid, keyHash: valid, expected: types.ErrInvalidProofType},
		{name: "short proof hash", proofType: types.ProofType_PROOF_TYPE_MINA_PICKLES, proofHash: valid[:31], publicInputsHash: valid, keyHash: valid, expected: types.ErrInvalidProofHash},
		{name: "long proof hash", proofType: types.ProofType_PROOF_TYPE_MINA_PICKLES, proofHash: append(bytes.Clone(valid), 1), publicInputsHash: valid, keyHash: valid, expected: types.ErrInvalidProofHash},
		{name: "short public inputs hash", proofType: types.ProofType_PROOF_TYPE_MINA_PICKLES, proofHash: valid, publicInputsHash: valid[:31], keyHash: valid, expected: types.ErrInvalidPublicInputsHash},
		{name: "long public inputs hash", proofType: types.ProofType_PROOF_TYPE_MINA_PICKLES, proofHash: valid, publicInputsHash: append(bytes.Clone(valid), 1), keyHash: valid, expected: types.ErrInvalidPublicInputsHash},
		{name: "short key hash", proofType: types.ProofType_PROOF_TYPE_MINA_PICKLES, proofHash: valid, publicInputsHash: valid, keyHash: valid[:31], expected: types.ErrInvalidVerificationKeyHash},
		{name: "long key hash", proofType: types.ProofType_PROOF_TYPE_MINA_PICKLES, proofHash: valid, publicInputsHash: valid, keyHash: append(bytes.Clone(valid), 1), expected: types.ErrInvalidVerificationKeyHash},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := types.ComputeVerificationID(testCase.proofType, testCase.proofHash, testCase.publicInputsHash, testCase.keyHash)
			require.ErrorIs(t, err, testCase.expected)
		})
	}
}

func TestValidateProofRecordRejectsMismatchedVerificationID(t *testing.T) {
	record := validProofRecord(t, 7)
	require.NoError(t, types.ValidateProofRecord(record))

	record.VerificationId = bytes.Clone(record.VerificationId)
	record.VerificationId[0] ^= 0xff
	require.ErrorIs(t, types.ValidateProofRecord(record), types.ErrInvalidVerificationID)
}
