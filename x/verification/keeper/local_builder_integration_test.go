package keeper_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/node101-io/pulsar-chain/x/verification/sidecar"
	sidecarv1 "github.com/node101-io/pulsar-chain/x/verification/sidecar/api/v1"
	"github.com/node101-io/pulsar-chain/x/verification/types"
	verificationvalidator "github.com/node101-io/pulsar-chain/x/verification/validator"
)

type resultService struct {
	sidecarv1.UnimplementedVerificationServiceServer
	getResults func(*sidecarv1.GetVerificationResultsRequest) *sidecarv1.GetVerificationResultsResponse
	statusCall bool
}

func (s *resultService) GetVerificationResults(
	_ context.Context,
	request *sidecarv1.GetVerificationResultsRequest,
) (*sidecarv1.GetVerificationResultsResponse, error) {
	return s.getResults(request), nil
}

func (s *resultService) GetProofStatuses(
	context.Context,
	*sidecarv1.GetProofStatusesRequest,
) (*sidecarv1.GetProofStatusesResponse, error) {
	s.statusCall = true
	return &sidecarv1.GetProofStatusesResponse{}, nil
}

func startResultService(t *testing.T, service sidecarv1.VerificationServiceServer) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	sidecarv1.RegisterVerificationServiceServer(server, service)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	return listener.Addr().String()
}

func TestLocalBuilderRestartRevealsIntoKeeperAndFinalizes(t *testing.T) {
	fixture := initFixture(t, 1)
	submitProof(t, fixture, 500, 1)
	require.NoError(t, fixture.keeper.EndBlock(fixture.atHeight(500)))
	proofs, err := fixture.keeper.GetProofsAtHeight(fixture.ctx, 500)
	require.NoError(t, err)
	verificationID := proofs[0].Record.VerificationId
	stateDirectory := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(stateDirectory, 0o700))
	stateStore, err := verificationvalidator.NewFileStore(filepath.Join(stateDirectory, "verification_state.json"))
	require.NoError(t, err)
	provider := sidecar.ProviderFunc(func(_ context.Context, ids [][]byte) ([]sidecar.VerificationResult, error) {
		require.Equal(t, [][]byte{verificationID}, ids)
		return []sidecar.VerificationResult{{VerificationID: bytes.Clone(ids[0]), Result: sidecar.ResultValid}}, nil
	})
	identity := verificationvalidator.Identity{
		ChainID: "restart-integration", OperatorAddress: fixture.validators[0].operator,
		ConsensusPublicKey: bytes.Repeat([]byte{9}, 32),
	}
	builder, err := verificationvalidator.NewBuilder(
		fixture.keeper, provider, stateStore, bytes.NewReader(bytes.Repeat([]byte{7}, 64)), 50*time.Millisecond,
	)
	require.NoError(t, err)

	// H+2 is the first commitment opportunity for the proof submitted at H=500.
	// The live sidecar returns a terminal VALID verdict, so the builder places the
	// vote in commitment 502's right leaf and persists its private salt and vote
	// before the root can be signed and applied on-chain.
	commitment := builder.Build(fixture.atHeight(502), identity, 502)
	require.NoError(t, commitment.Warning)
	require.NotNil(t, commitment.Payload)
	require.NoError(t, fixture.keeper.ApplyVerificationPayload(
		fixture.atHeight(502), identity.OperatorAddress, 502, commitment.Payload.Commitment, nil,
	))

	// Restart before reveal with no sidecar available. The replacement builder
	// must recover the exact salt and vote from disk and reveal commitment 502 at
	// H+4. This proves revelation depends on durable local preimages, not on
	// re-running a verifier or receiving the same result after restart.
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
	require.Equal(t, int64(100), tally.ValidVotingPower)

	// EndBlock(H+5) runs after the last legal reveal block and finalizes from the
	// historical power snapshot frozen at H. With one validator holding all 100
	// power, the vote strictly exceeds two thirds and produces a VALID result.
	require.NoError(t, fixture.keeper.EndBlock(fixture.atHeight(505)))
	result, err := fixture.keeper.FinalProofResults.Get(fixture.ctx, types.NewProofStoreKey(500, 0))
	require.NoError(t, err)
	require.Equal(t, types.ProofStatus_PROOF_STATUS_VALID, result.Status)
	require.Equal(t, int64(67), result.VotingPowerThreshold)
}

func TestGRPCClientFeedsOnlyNewTerminalResultsIntoCommitments(t *testing.T) {
	fixture := initFixture(t, 1)
	for hashByte := byte(1); hashByte <= 3; hashByte++ {
		submitProof(t, fixture, 500, hashByte)
	}
	require.NoError(t, fixture.keeper.EndBlock(fixture.atHeight(500)))
	proofs, err := fixture.keeper.GetProofsAtHeight(fixture.ctx, 500)
	require.NoError(t, err)
	call := 0
	service := &resultService{getResults: func(
		request *sidecarv1.GetVerificationResultsRequest,
	) *sidecarv1.GetVerificationResultsResponse {
		call++
		switch call {
		case 1:
			// H+2 is the first opportunity for all proofs from block H. The request
			// contains the full batch, but only proof 0 has a terminal result; proofs
			// 1 and 2 are omitted rather than mapped to INVALID.
			require.Equal(t, [][]byte{
				proofs[0].Record.VerificationId,
				proofs[1].Record.VerificationId,
				proofs[2].Record.VerificationId,
			}, request.VerificationIds)
			return &sidecarv1.GetVerificationResultsResponse{Results: []*sidecarv1.VerificationResult{{
				VerificationId: proofs[0].Record.VerificationId,
				Verdict:        sidecarv1.VerificationVerdict_VERIFICATION_VERDICT_VALID,
			}}}
		case 2:
			// H+3 is the overlapping second opportunity for block H. Proof 0 is
			// excluded because its vote is already protected by the on-chain H+2
			// commitment. Proof 1 has now completed; proof 2 remains pending and
			// never becomes an implicit negative vote.
			require.Equal(t, [][]byte{
				proofs[1].Record.VerificationId,
				proofs[2].Record.VerificationId,
			}, request.VerificationIds)
			return &sidecarv1.GetVerificationResultsResponse{Results: []*sidecarv1.VerificationResult{{
				VerificationId: proofs[1].Record.VerificationId,
				Verdict:        sidecarv1.VerificationVerdict_VERIFICATION_VERDICT_INVALID,
			}}}
		default:
			t.Fatalf("unexpected result RPC call %d", call)
			return nil
		}
	}}
	client, err := sidecar.NewClient(startResultService(t, service), sidecar.TransportModeLoopback)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	builder, err := verificationvalidator.NewBuilder(
		fixture.keeper,
		client,
		verificationvalidator.NewMemoryStore(),
		bytes.NewReader(bytes.Repeat([]byte{7}, 128)),
		50*time.Millisecond,
	)
	require.NoError(t, err)
	identity := verificationvalidator.Identity{
		ChainID: "grpc-integration", OperatorAddress: fixture.validators[0].operator,
		ConsensusPublicKey: bytes.Repeat([]byte{9}, 32),
	}

	first := builder.Build(fixture.atHeight(502), identity, 502)
	require.NoError(t, first.Warning)
	require.NotNil(t, first.Payload)
	require.NoError(t, fixture.keeper.ApplyVerificationPayload(
		fixture.atHeight(502), identity.OperatorAddress, 502, first.Payload.Commitment, nil,
	))
	second := builder.Build(fixture.atHeight(503), identity, 503)
	require.NoError(t, second.Warning)
	require.NotNil(t, second.Payload)
	require.Equal(t, 2, call)
	// The consensus client must never query sidecar-local lifecycle status.
	// QUEUED, VERIFYING, and FAILED are operational observations; only terminal
	// VALID or INVALID results can influence a commitment.
	require.False(t, service.statusCall)
}
