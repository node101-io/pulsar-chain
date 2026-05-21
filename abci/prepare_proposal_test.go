package abci

import (
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

	payload, err := handler.constructPayload(ctx, height, votes)
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
