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

func TestGRPCClientFeedsOnlyNewTerminalResultsIntoCommitments(t *testing.T) {
	fixture := initFixture(t, 1)
	for hashByte := byte(1); hashByte <= 3; hashByte++ {
		submitProof(t, fixture, 500, hashByte)
	}
	proofs, err := fixture.keeper.GetProofsAtHeight(fixture.ctx, 500)
	require.NoError(t, err)
	call := 0
	service := &resultService{getResults: func(
		request *sidecarv1.GetVerificationResultsRequest,
	) *sidecarv1.GetVerificationResultsResponse {
		call++
		switch call {
		case 1:
			require.Equal(t, [][]byte{
				proofs[0].Record.ProofHash,
				proofs[1].Record.ProofHash,
				proofs[2].Record.ProofHash,
			}, request.ProofHashes)
			return &sidecarv1.GetVerificationResultsResponse{Results: []*sidecarv1.VerificationResult{{
				ProofHash: proofs[0].Record.ProofHash,
				Verdict:   sidecarv1.VerificationVerdict_VERIFICATION_VERDICT_VALID,
			}}}
		case 2:
			require.Equal(t, [][]byte{
				proofs[1].Record.ProofHash,
				proofs[2].Record.ProofHash,
			}, request.ProofHashes)
			return &sidecarv1.GetVerificationResultsResponse{Results: []*sidecarv1.VerificationResult{{
				ProofHash: proofs[1].Record.ProofHash,
				Verdict:   sidecarv1.VerificationVerdict_VERIFICATION_VERDICT_INVALID,
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
	require.False(t, service.statusCall)
}
