package abci

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	cometabci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestPrepareProposalReturnsOriginalTxsWhenVoteExtensionsDisabled(t *testing.T) {
	handler := &ABCIHandler{}
	ctx := sdk.Context{}.WithConsensusParams(tmproto.ConsensusParams{
		Abci: &tmproto.ABCIParams{VoteExtensionsEnableHeight: 100},
	})
	req := &cometabci.RequestPrepareProposal{
		Height:     10,
		MaxTxBytes: 100,
		Txs:        [][]byte{[]byte("tx-1"), []byte("tx-2")},
	}

	response, err := handler.PrepareProposalHandler()(ctx, req)

	require.NoError(t, err)
	require.Equal(t, req.Txs, response.Txs)
}

func TestPrepareProposalReturnsNilResponseOnPayloadConstructionError(t *testing.T) {
	handler := &ABCIHandler{stakingKeeper: failingPrepareProposalStakingKeeper{}}
	ctx := prepareProposalTestContext(10)
	req := &cometabci.RequestPrepareProposal{
		Height:     10,
		MaxTxBytes: 100,
	}

	response, err := handler.PrepareProposalHandler()(ctx, req)

	require.Nil(t, response)
	require.Error(t, err)
}

func TestPrepareProposalRejectsEmptyPayload(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, nil, nil)
	ctx := prepareProposalTestContext(10)
	req := &cometabci.RequestPrepareProposal{
		Height:     10,
		MaxTxBytes: 100,
		LocalLastCommit: cometabci.ExtendedCommitInfo{
			Votes: []cometabci.ExtendedVoteInfo{prepareProposalVote(t, validator, nil, tmproto.BlockIDFlagCommit)},
		},
	}

	response, err := handler.PrepareProposalHandler()(ctx, req)

	require.Nil(t, response)
	require.ErrorIs(t, err, ErrNoVoteExtensionsForPayload)
}

func TestPrepareProposalRejectsPayloadLargerThanMaxTxBytes(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, nil, nil)
	ctx := prepareProposalTestContext(10)
	req := &cometabci.RequestPrepareProposal{
		Height:     10,
		MaxTxBytes: 1,
		LocalLastCommit: cometabci.ExtendedCommitInfo{
			Votes: []cometabci.ExtendedVoteInfo{prepareProposalVote(t, validator, []byte("vote-extension"), tmproto.BlockIDFlagCommit)},
		},
	}

	response, err := handler.PrepareProposalHandler()(ctx, req)

	require.Nil(t, response)
	require.ErrorIs(t, err, ErrVoteExtPayloadTooLarge)
}

func TestPrepareProposalPrependsPayloadAndKeepsAllUserTxsWithinMaxBytes(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, nil, nil)
	ctx := prepareProposalTestContext(10)
	req := &cometabci.RequestPrepareProposal{
		Height:     10,
		MaxTxBytes: 10_000,
		Txs:        [][]byte{[]byte("tx-1"), []byte("tx-2")},
		LocalLastCommit: cometabci.ExtendedCommitInfo{
			Votes: []cometabci.ExtendedVoteInfo{prepareProposalVote(t, validator, []byte("vote-extension"), tmproto.BlockIDFlagCommit)},
		},
	}

	response, err := handler.PrepareProposalHandler()(ctx, req)

	require.NoError(t, err)
	require.Len(t, response.Txs, 3)
	require.True(t, hasVoteExtMarker(response.Txs[0]))
	require.Equal(t, req.Txs, response.Txs[1:])
}

func TestPrepareProposalKeepsAllUserTxsWhenMaxTxBytesIsUnlimited(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, nil, nil)
	ctx := prepareProposalTestContext(10)
	req := &cometabci.RequestPrepareProposal{
		Height:     10,
		MaxTxBytes: -1,
		Txs:        [][]byte{[]byte("tx-1"), []byte("tx-2")},
		LocalLastCommit: cometabci.ExtendedCommitInfo{
			Votes: []cometabci.ExtendedVoteInfo{prepareProposalVote(t, validator, []byte("vote-extension"), tmproto.BlockIDFlagCommit)},
		},
	}

	response, err := handler.PrepareProposalHandler()(ctx, req)

	require.NoError(t, err)
	require.Len(t, response.Txs, 3)
	require.True(t, hasVoteExtMarker(response.Txs[0]))
	require.Equal(t, req.Txs, response.Txs[1:])
}

func TestPrepareProposalTrimsUserTxsToMaxTxBytes(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, nil, nil)
	ctx := prepareProposalTestContext(10)
	req := &cometabci.RequestPrepareProposal{
		Height: 10,
		Txs:    [][]byte{[]byte("tx-1"), []byte("tx-2")},
		LocalLastCommit: cometabci.ExtendedCommitInfo{
			Votes: []cometabci.ExtendedVoteInfo{prepareProposalVote(t, validator, []byte("vote-extension"), tmproto.BlockIDFlagCommit)},
		},
	}
	req.MaxTxBytes = int64(len(prepareProposalPayloadTx(t, handler, ctx, req.GetHeight(), req.LocalLastCommit.Votes)) + len(req.Txs[0]))

	response, err := handler.PrepareProposalHandler()(ctx, req)

	require.NoError(t, err)
	require.Len(t, response.Txs, 2)
	require.True(t, hasVoteExtMarker(response.Txs[0]))
	require.Equal(t, req.Txs[:1], response.Txs[1:])
}

func TestPrepareProposalReturnsOnlyPayloadWhenFirstUserTxDoesNotFit(t *testing.T) {
	validator := newTestBondedValidator(t, 10)
	handler := newQuorumTestHandler(t, []stakingtypes.Validator{validator}, nil, nil)
	ctx := prepareProposalTestContext(10)
	req := &cometabci.RequestPrepareProposal{
		Height: 10,
		Txs:    [][]byte{[]byte("tx-1")},
		LocalLastCommit: cometabci.ExtendedCommitInfo{
			Votes: []cometabci.ExtendedVoteInfo{prepareProposalVote(t, validator, []byte("vote-extension"), tmproto.BlockIDFlagCommit)},
		},
	}
	req.MaxTxBytes = int64(len(prepareProposalPayloadTx(t, handler, ctx, req.GetHeight(), req.LocalLastCommit.Votes)))

	response, err := handler.PrepareProposalHandler()(ctx, req)

	require.NoError(t, err)
	require.Len(t, response.Txs, 1)
	require.True(t, hasVoteExtMarker(response.Txs[0]))
}

func TestFitVerificationEntriesUsesExactWireSizeAndCanonicalOutput(t *testing.T) {
	payload := Payload{
		VoteExtensionHeight: 9,
		VoteExtensions: []*PayloadVoteExtension{{
			ConsensusPublicKey: []byte{1}, VoteExtension: []byte{2},
		}},
		VerificationEntries: []*PayloadVerificationEntry{
			{ValidatorAddress: []byte{3}, CompositeVoteExtension: bytes.Repeat([]byte{3}, 20)},
			{ValidatorAddress: []byte{1}, CompositeVoteExtension: bytes.Repeat([]byte{1}, 20)},
			{ValidatorAddress: []byte{2}, CompositeVoteExtension: bytes.Repeat([]byte{2}, 20)},
		},
	}
	unlimited, err := fitVerificationEntries(payload, 10, -1)
	require.NoError(t, err)
	require.Len(t, unlimited.VerificationEntries, 3)
	require.Equal(t, []byte{1}, unlimited.VerificationEntries[0].ValidatorAddress)
	require.Equal(t, []byte{2}, unlimited.VerificationEntries[1].ValidatorAddress)
	require.Equal(t, []byte{3}, unlimited.VerificationEntries[2].ValidatorAddress)

	base := payload
	base.VerificationEntries = nil
	firstRotated := payload.VerificationEntries[1]
	entrySize := firstRotated.Size()
	maxBytes := int64(len(voteExtMarkerBytes) + base.Size() + 1 + protobufVarintSize(uint64(entrySize)) + entrySize)
	limited, err := fitVerificationEntries(payload, 10, maxBytes)
	require.NoError(t, err)
	require.Len(t, limited.VerificationEntries, 1)
	require.Equal(t, firstRotated.ValidatorAddress, limited.VerificationEntries[0].ValidatorAddress)
	encoded, err := limited.Marshal()
	require.NoError(t, err)
	require.Equal(t, maxBytes, int64(len(voteExtMarkerBytes)+len(encoded)))
}

func prepareProposalTestContext(blockHeight int64) sdk.Context {
	return sdk.Context{}.
		WithBlockHeight(blockHeight).
		WithConsensusParams(tmproto.ConsensusParams{
			Abci: &tmproto.ABCIParams{VoteExtensionsEnableHeight: 1},
		})
}

func prepareProposalVote(t *testing.T, validator stakingtypes.Validator, voteExtension []byte, blockIDFlag tmproto.BlockIDFlag) cometabci.ExtendedVoteInfo {
	t.Helper()

	consAddr, err := validator.GetConsAddr()
	require.NoError(t, err)
	if len(voteExtension) != 0 {
		voteExtension, err = encodeCompositeVoteExtension(&CompositeVoteExtension{
			ProtocolVersion:     CompositeVoteExtensionVersion,
			TransitionSignature: voteExtension,
		})
		require.NoError(t, err)
	}

	return cometabci.ExtendedVoteInfo{
		Validator: cometabci.Validator{
			Address: consAddr,
		},
		VoteExtension: voteExtension,
		BlockIdFlag:   blockIDFlag,
	}
}

func prepareProposalPayloadTx(t *testing.T, handler *ABCIHandler, ctx sdk.Context, height int64, votes []cometabci.ExtendedVoteInfo) []byte {
	t.Helper()

	payload, err := handler.constructPayload(ctx, height, 0, votes)
	require.NoError(t, err)

	payloadBytes, err := payload.Marshal()
	require.NoError(t, err)

	return append(voteExtMarkerBytes[:len(voteExtMarkerBytes):len(voteExtMarkerBytes)], payloadBytes...)
}

func hasVoteExtMarker(tx []byte) bool {
	return len(tx) >= len(voteExtMarkerBytes) && string(tx[:len(voteExtMarkerBytes)]) == VoteExtMarker
}

type failingPrepareProposalStakingKeeper struct{}

func (failingPrepareProposalStakingKeeper) IterateLastValidators(context.Context, func(int64, stakingtypes.ValidatorI) bool) error {
	return fmt.Errorf("iterate validators failed")
}

func (failingPrepareProposalStakingKeeper) GetHistoricalInfo(context.Context, int64) (stakingtypes.HistoricalInfo, error) {
	return stakingtypes.HistoricalInfo{}, fmt.Errorf("historical info failed")
}

func (failingPrepareProposalStakingKeeper) GetValidatorByConsAddr(context.Context, sdk.ConsAddress) (stakingtypes.Validator, error) {
	return stakingtypes.Validator{}, fmt.Errorf("validator not found")
}
