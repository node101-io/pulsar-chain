package keeper_test

import (
	"bytes"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

	storedFirstProof, err := f.keeper.GetPendingProof(f.ctx, types.ProofID{
		BlockHeight: currentBlockHeight,
		ProofIndex:  0,
	})
	require.NoError(t, err)
	require.Equal(t, firstProof, storedFirstProof)

	storedSecondProof, err := f.keeper.GetPendingProof(f.ctx, types.ProofID{
		BlockHeight: currentBlockHeight,
		ProofIndex:  1,
	})
	require.NoError(t, err)
	require.Equal(t, secondProof, storedSecondProof)

	oldBlockExists, err := f.keeper.PendingProofBlockExists(f.ctx, oldBlockHeight)
	require.NoError(t, err)
	require.False(t, oldBlockExists)
}

func TestGetProofHashesByBlockHeight(t *testing.T) {
	f := initFixture(t)
	blockHeight := int64(20)
	firstProof := []byte("proof-0")
	secondProof := []byte("proof-1")

	require.NoError(t, f.keeper.AppendPendingProof(f.ctx, firstProof, blockHeight))
	require.NoError(t, f.keeper.AppendPendingProof(f.ctx, secondProof, blockHeight))
	require.NoError(t, f.keeper.AppendPendingProof(f.ctx, []byte("next-block-proof"), blockHeight+1))

	proofHashes, proofIDs, err := f.keeper.GetProofHashesByBlockHeight(f.ctx, blockHeight)
	require.NoError(t, err)
	require.Equal(t, [][]byte{firstProof, secondProof}, proofHashes)
	require.Equal(t, []types.ProofID{
		{BlockHeight: blockHeight, ProofIndex: 0},
		{BlockHeight: blockHeight, ProofIndex: 1},
	}, proofIDs)
}

func TestAppendPendingProofRejectsBlockProofLimit(t *testing.T) {
	f := initFixture(t)
	require.NoError(t, f.keeper.Params.Set(f.ctx, types.NewParams(6, 2)))

	require.NoError(t, f.keeper.AppendPendingProof(f.ctx, []byte("proof-0"), 20))
	require.NoError(t, f.keeper.AppendPendingProof(f.ctx, []byte("proof-1"), 20))

	err := f.keeper.AppendPendingProof(f.ctx, []byte("proof-2"), 20)
	require.ErrorIs(t, err, types.ErrFailedToAppendPendingProof)
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
		storedProofHash, err := f.keeper.GetPendingProof(ctx, types.ProofID{
			BlockHeight: blockHeight,
			ProofIndex:  int64(index),
		})
		require.NoError(t, err)
		require.Equal(t, proofHash, storedProofHash)
	}
}

func TestPushNewProofHashErrors(t *testing.T) {
	tests := []struct {
		name         string
		message      func(string) *types.MsgPushNewProofHash
		removeParams bool
		expectedCode codes.Code
		expectedText string
	}{
		{
			name: "nil request",
			message: func(string) *types.MsgPushNewProofHash {
				return nil
			},
			expectedCode: codes.InvalidArgument,
			expectedText: types.ErrInvalidPushNewProofHashRequest.Error(),
		},
		{
			name: "invalid creator address",
			message: func(string) *types.MsgPushNewProofHash {
				return &types.MsgPushNewProofHash{
					Creator:   "invalid",
					ProofHash: bytes.Repeat([]byte{0x11}, 32),
				}
			},
			expectedCode: codes.InvalidArgument,
			expectedText: types.ErrInvalidCreatorAddress.Error(),
		},
		{
			name: "empty proof hash",
			message: func(creator string) *types.MsgPushNewProofHash {
				return &types.MsgPushNewProofHash{Creator: creator}
			},
			expectedCode: codes.InvalidArgument,
			expectedText: types.ErrInvalidProofHashLength.Error(),
		},
		{
			name: "invalid proof hash length",
			message: func(creator string) *types.MsgPushNewProofHash {
				return &types.MsgPushNewProofHash{
					Creator:   creator,
					ProofHash: bytes.Repeat([]byte{0x11}, 31),
				}
			},
			expectedCode: codes.InvalidArgument,
			expectedText: types.ErrInvalidProofHashLength.Error(),
		},
		{
			name: "append pending proof failure",
			message: func(creator string) *types.MsgPushNewProofHash {
				return &types.MsgPushNewProofHash{
					Creator:   creator,
					ProofHash: bytes.Repeat([]byte{0x11}, 32),
				}
			},
			removeParams: true,
			expectedCode: codes.Internal,
			expectedText: types.ErrFailedToAppendPendingProof.Error(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := initFixture(t)
			ctx := sdk.UnwrapSDKContext(f.ctx).WithBlockHeight(42)
			creator, err := f.addressCodec.BytesToString(f.keeper.GetAuthority())
			require.NoError(t, err)

			if test.removeParams {
				require.NoError(t, f.keeper.Params.Remove(ctx))
			}

			_, err = keeper.NewMsgServerImpl(f.keeper).PushNewProofHash(ctx, test.message(creator))
			require.Error(t, err)
			require.Equal(t, test.expectedCode, status.Code(err))
			require.ErrorContains(t, err, test.expectedText)
		})
	}
}
