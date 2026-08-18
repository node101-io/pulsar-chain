package keeper_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func TestBuilderReadsProofsAndCommitmentDefensively(t *testing.T) {
	f := initFixture(t, 1)
	proofHash := bytes.Repeat([]byte{0x31}, types.ProofHashSize)
	_, err := f.msgServer.SubmitProof(f.atHeight(5), &types.MsgSubmitProof{
		Signer: f.validators[0].signer, ProofHash: proofHash, ProofType: 7,
	})
	require.NoError(t, err)

	proofs, err := f.keeper.GetProofsAtHeight(f.ctx, 5)
	require.NoError(t, err)
	require.Len(t, proofs, 1)
	require.Equal(t, proofHash, proofs[0].Record.ProofHash)
	proofs[0].Record.ProofHash[0] ^= 0xff

	stored, err := f.keeper.PendingProofs.Get(f.ctx, types.NewProofStoreKey(5, 0))
	require.NoError(t, err)
	require.Equal(t, proofHash, stored.ProofHash)

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
