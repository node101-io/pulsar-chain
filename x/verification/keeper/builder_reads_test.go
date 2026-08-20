package keeper_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func TestBuilderReadsProofsAndCommitmentDefensively(t *testing.T) {
	f := initFixture(t, 1)
	msg := proofSubmission(f, 0x31)
	proofHash := msg.ProofHash
	_, err := f.msgServer.SubmitProof(f.atHeight(5), msg)
	require.NoError(t, err)

	proofs, err := f.keeper.GetProofsAtHeight(f.ctx, 5)
	require.NoError(t, err)
	require.Len(t, proofs, 1)
	require.Equal(t, proofHash, proofs[0].Record.ProofHash)
	proofs[0].Record.ProofHash[0] ^= 0xff
	proofs[0].Record.PublicInputsHash[0] ^= 0xff
	proofs[0].Record.VerificationKeyHash[0] ^= 0xff
	proofs[0].Record.VerificationId[0] ^= 0xff

	stored, err := f.keeper.PendingProofs.Get(f.ctx, types.NewProofStoreKey(5, 0))
	require.NoError(t, err)
	require.Equal(t, proofHash, stored.ProofHash)
	require.Equal(t, msg.PublicInputsHash, stored.PublicInputsHash)
	require.Equal(t, msg.VerificationKeyHash, stored.VerificationKeyHash)

	validator := f.validators[0].operator
	root := bytes.Repeat([]byte{0x42}, types.CommitmentHashSize)
	require.NoError(t, f.keeper.Commitments.Set(f.ctx, types.NewCommitmentStoreKey(validator, 10), root))
	got, found, err := f.keeper.GetCommitment(f.ctx, validator, 10)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, root, got)
	got[0] ^= 0xff
	again, found, err := f.keeper.GetCommitment(f.ctx, validator, 10)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, root, again)
}

func TestBuilderReadsMissingHeightAndCommitment(t *testing.T) {
	f := initFixture(t, 1)
	proofs, err := f.keeper.GetProofsAtHeight(f.ctx, 999)
	require.NoError(t, err)
	require.Empty(t, proofs)

	root, found, err := f.keeper.GetCommitment(f.ctx, f.validators[0].operator, 999)
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, root)
}

func TestBuilderReadsRejectMismatchedVerificationID(t *testing.T) {
	f := initFixture(t, 1)
	submitProof(t, f, 5, 1)
	key := types.NewProofStoreKey(5, 0)
	record, err := f.keeper.PendingProofs.Get(f.ctx, key)
	require.NoError(t, err)
	record.VerificationId[0] ^= 0xff
	require.NoError(t, f.keeper.PendingProofs.Set(f.ctx, key, record))

	_, err = f.keeper.GetProofsAtHeight(f.ctx, 5)
	require.ErrorIs(t, err, types.ErrProofStateCorrupted)
}
