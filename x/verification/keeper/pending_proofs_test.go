package keeper_test

import (
	"bytes"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/verification/keeper"
	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func TestPendingProofBlockExists(t *testing.T) {
	f := initFixture(t)
	blockHeight := int64(20)

	exists, err := f.keeper.PendingProofBlockExists(f.ctx, blockHeight)
	require.NoError(t, err)
	require.False(t, exists)

	require.NoError(t, f.keeper.AppendPendingProof(f.ctx, []byte("proof"), blockHeight))

	exists, err = f.keeper.PendingProofBlockExists(f.ctx, blockHeight)
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = f.keeper.PendingProofBlockExists(f.ctx, blockHeight+1)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestGetNextPendingProofIndex(t *testing.T) {
	f := initFixture(t)
	blockHeight := int64(20)

	nextIndex, err := f.keeper.GetNextPendingProofIndex(f.ctx, blockHeight)
	require.NoError(t, err)
	require.Equal(t, int64(0), nextIndex)

	require.NoError(t, f.keeper.AppendPendingProof(f.ctx, []byte("proof-0"), blockHeight))
	nextIndex, err = f.keeper.GetNextPendingProofIndex(f.ctx, blockHeight)
	require.NoError(t, err)
	require.Equal(t, int64(1), nextIndex)

	require.NoError(t, f.keeper.AppendPendingProof(f.ctx, []byte("proof-1"), blockHeight))
	nextIndex, err = f.keeper.GetNextPendingProofIndex(f.ctx, blockHeight)
	require.NoError(t, err)
	require.Equal(t, int64(2), nextIndex)
}

func TestAppendPendingProof(t *testing.T) {
	f := initFixture(t)
	oldBlockHeight := int64(10)
	currentBlockHeight := int64(16)
	firstProof := []byte("first-proof")
	secondProof := []byte("second-proof")

	require.NoError(t, f.keeper.AppendPendingProof(f.ctx, []byte("old-proof"), oldBlockHeight))
	require.NoError(t, f.keeper.AppendPendingProof(f.ctx, firstProof, currentBlockHeight))
	require.NoError(t, f.keeper.AppendPendingProof(f.ctx, secondProof, currentBlockHeight))

	storedFirstProof, err := f.keeper.GetPendingProof(f.ctx, currentBlockHeight, 0)
	require.NoError(t, err)
	require.Equal(t, firstProof, storedFirstProof)

	storedSecondProof, err := f.keeper.GetPendingProof(f.ctx, currentBlockHeight, 1)
	require.NoError(t, err)
	require.Equal(t, secondProof, storedSecondProof)

	oldBlockExists, err := f.keeper.PendingProofBlockExists(f.ctx, oldBlockHeight)
	require.NoError(t, err)
	require.False(t, oldBlockExists)
}

func TestPushNewProofHash(t *testing.T) {
	f := initFixture(t)
	blockHeight := int64(42)
	ctx := sdk.UnwrapSDKContext(f.ctx).WithBlockHeight(blockHeight)
	creator, err := f.addressCodec.BytesToString(f.keeper.GetAuthority())
	require.NoError(t, err)

	proofHashes := [][]byte{
		bytes.Repeat([]byte{0x11}, 32),
		bytes.Repeat([]byte{0x22}, 32),
	}
	msgServer := keeper.NewMsgServerImpl(f.keeper)

	for _, proofHash := range proofHashes {
		response, err := msgServer.PushNewProofHash(ctx, &types.MsgPushNewProofHash{
			Creator:   creator,
			ProofHash: proofHash,
		})
		require.NoError(t, err)
		require.Equal(t, &types.MsgPushNewProofHashResponse{}, response)
	}

	for index, proofHash := range proofHashes {
		storedProofHash, err := f.keeper.GetPendingProof(ctx, blockHeight, int64(index))
		require.NoError(t, err)
		require.Equal(t, proofHash, storedProofHash)
	}
}
