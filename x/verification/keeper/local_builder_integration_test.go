package keeper_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/verification/sidecar"
	"github.com/node101-io/pulsar-chain/x/verification/types"
	verificationvalidator "github.com/node101-io/pulsar-chain/x/verification/validator"
)

func TestLocalBuilderRestartRevealsIntoKeeperAndFinalizes(t *testing.T) {
	fixture := initFixture(t, 1)
	submitProof(t, fixture, 500, 1)
	proofHash := make([]byte, types.ProofHashSize)
	proofHash[len(proofHash)-1] = 1
	stateDirectory := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(stateDirectory, 0o700))
	stateStore, err := verificationvalidator.NewFileStore(filepath.Join(stateDirectory, "verification_state.json"))
	require.NoError(t, err)
	provider := sidecar.ProviderFunc(func(_ context.Context, hashes [][]byte) ([]sidecar.VerificationResult, error) {
		require.Equal(t, [][]byte{proofHash}, hashes)
		return []sidecar.VerificationResult{{ProofHash: bytes.Clone(hashes[0]), Result: sidecar.ResultValid}}, nil
	})
	identity := verificationvalidator.Identity{
		ChainID: "restart-integration", OperatorAddress: fixture.validators[0].operator,
		ConsensusPublicKey: bytes.Repeat([]byte{9}, 32),
	}
	builder, err := verificationvalidator.NewBuilder(
		fixture.keeper, provider, stateStore, bytes.NewReader(bytes.Repeat([]byte{7}, 64)), 50*time.Millisecond,
	)
	require.NoError(t, err)

	commitment := builder.Build(fixture.atHeight(502), identity, 502)
	require.NoError(t, commitment.Warning)
	require.NotNil(t, commitment.Payload)
	require.NoError(t, fixture.keeper.ApplyVerificationPayload(
		fixture.atHeight(502), identity.OperatorAddress, 502, commitment.Payload.Commitment, nil,
	))

	restarted, err := verificationvalidator.NewBuilder(
		fixture.keeper, sidecar.DisabledProvider{}, stateStore, bytes.NewReader(nil), 50*time.Millisecond,
	)
	require.NoError(t, err)
	revelation := restarted.Build(fixture.atHeight(504), identity, 504)
	require.NoError(t, revelation.Warning)
	require.NotNil(t, revelation.Payload)
	require.Empty(t, revelation.Payload.Commitment)
	require.Len(t, revelation.Payload.Revelations, 1)
	require.Equal(t, []types.ProofVote{{IndexInBlock: 0, Result: true}}, revelation.Payload.Revelations[0].Right.GetValue().Votes)
	require.NoError(t, fixture.keeper.ApplyVerificationPayload(
		fixture.atHeight(504), identity.OperatorAddress, 504, nil, revelation.Payload.Revelations,
	))
	tally, err := fixture.keeper.ProofTallies.Get(fixture.ctx, types.NewProofStoreKey(500, 0))
	require.NoError(t, err)
	require.Equal(t, uint32(1), tally.TrueVotes)

	require.NoError(t, fixture.keeper.EndBlock(fixture.atHeight(505)))
	result, err := fixture.keeper.FinalProofResults.Get(fixture.ctx, types.NewProofStoreKey(500, 0))
	require.NoError(t, err)
	require.Equal(t, types.ProofStatus_PROOF_STATUS_VALID, result.Status)
	require.Equal(t, uint32(1), result.Threshold)
}
