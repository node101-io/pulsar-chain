package sidecar

import (
	"bytes"
	"context"
	"net"
	"sync"
	"testing"
	"time"

	sidecarv1 "github.com/node101-io/pulsar-chain/x/verification/sidecar/api/v1"
	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type testVerificationServer struct {
	sidecarv1.UnimplementedVerificationServiceServer

	mu          sync.Mutex
	requests    []*sidecarv1.GetVerificationResultsRequest
	response    *sidecarv1.GetVerificationResultsResponse
	resultErr   error
	resultHook  func(context.Context, *sidecarv1.GetVerificationResultsRequest) (*sidecarv1.GetVerificationResultsResponse, error)
	statusCalls int
}

func (s *testVerificationServer) GetVerificationResults(
	ctx context.Context,
	request *sidecarv1.GetVerificationResultsRequest,
) (*sidecarv1.GetVerificationResultsResponse, error) {
	s.mu.Lock()
	s.requests = append(s.requests, request)
	hook, response, err := s.resultHook, s.response, s.resultErr
	s.mu.Unlock()
	if hook != nil {
		return hook(ctx, request)
	}
	return response, err
}

func (s *testVerificationServer) GetProofStatuses(
	context.Context,
	*sidecarv1.GetProofStatusesRequest,
) (*sidecarv1.GetProofStatusesResponse, error) {
	s.mu.Lock()
	s.statusCalls++
	s.mu.Unlock()
	return &sidecarv1.GetProofStatusesResponse{}, nil
}

func startVerificationServer(t *testing.T, service sidecarv1.VerificationServiceServer) string {
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

func verificationID(value byte) []byte {
	return bytes.Repeat([]byte{value}, verificationtypes.VerificationIDSize)
}

func TestClientReturnsTerminalSubsetInRequestOrder(t *testing.T) {
	ids := [][]byte{verificationID(1), verificationID(2), verificationID(3)}
	service := &testVerificationServer{response: &sidecarv1.GetVerificationResultsResponse{Results: []*sidecarv1.VerificationResult{
		{VerificationId: bytes.Clone(ids[2]), Verdict: sidecarv1.VerificationVerdict_VERIFICATION_VERDICT_INVALID},
		{VerificationId: bytes.Clone(ids[0]), Verdict: sidecarv1.VerificationVerdict_VERIFICATION_VERDICT_VALID},
	}}}
	client, err := NewClient(startVerificationServer(t, service), TransportModeLoopback)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	results, err := client.GetVerificationResults(context.Background(), ids)
	require.NoError(t, err)
	require.Equal(t, []VerificationResult{
		{VerificationID: ids[0], Result: ResultValid},
		{VerificationID: ids[2], Result: ResultInvalid},
	}, results)
	service.mu.Lock()
	require.Equal(t, ids, service.requests[0].VerificationIds)
	require.Zero(t, service.statusCalls)
	service.mu.Unlock()
}

func TestClientEmptyRequestDoesNotCallRPC(t *testing.T) {
	service := &testVerificationServer{}
	client, err := NewClient(startVerificationServer(t, service), TransportModeLoopback)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	results, err := client.GetVerificationResults(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, results)
	service.mu.Lock()
	require.Empty(t, service.requests)
	service.mu.Unlock()
}

func TestClientRejectsMalformedRequest(t *testing.T) {
	tooMany := make([][]byte, maxVerificationIDs+1)
	for i := range tooMany {
		id := make([]byte, verificationtypes.VerificationIDSize)
		id[0] = byte(i >> 8)
		id[1] = byte(i)
		tooMany[i] = id
	}
	testCases := []struct {
		name   string
		hashes [][]byte
	}{
		{name: "short ID", hashes: [][]byte{{1}}},
		{name: "duplicate", hashes: [][]byte{verificationID(1), verificationID(1)}},
		{name: "too many", hashes: tooMany},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var client *Client
			_, err := client.GetVerificationResults(context.Background(), testCase.hashes)
			require.ErrorIs(t, err, ErrInvalidRequest)
		})
	}
}

func TestValidateResponseRejectsMalformedResults(t *testing.T) {
	id := verificationID(1)
	other := verificationID(2)
	_, requested, err := buildRequest([][]byte{id})
	require.NoError(t, err)
	valid := &sidecarv1.VerificationResult{
		VerificationId: bytes.Clone(id), Verdict: sidecarv1.VerificationVerdict_VERIFICATION_VERDICT_VALID,
	}
	testCases := []struct {
		name     string
		response *sidecarv1.GetVerificationResultsResponse
	}{
		{name: "nil response"},
		{name: "nil result", response: &sidecarv1.GetVerificationResultsResponse{Results: []*sidecarv1.VerificationResult{nil}}},
		{name: "short ID", response: &sidecarv1.GetVerificationResultsResponse{Results: []*sidecarv1.VerificationResult{{VerificationId: []byte{1}, Verdict: sidecarv1.VerificationVerdict_VERIFICATION_VERDICT_VALID}}}},
		{name: "unrequested", response: &sidecarv1.GetVerificationResultsResponse{Results: []*sidecarv1.VerificationResult{{VerificationId: other, Verdict: sidecarv1.VerificationVerdict_VERIFICATION_VERDICT_VALID}}}},
		{name: "duplicate", response: &sidecarv1.GetVerificationResultsResponse{Results: []*sidecarv1.VerificationResult{valid, valid}}},
		{name: "unspecified", response: &sidecarv1.GetVerificationResultsResponse{Results: []*sidecarv1.VerificationResult{{VerificationId: id}}}},
		{name: "unknown verdict", response: &sidecarv1.GetVerificationResultsResponse{Results: []*sidecarv1.VerificationResult{{VerificationId: id, Verdict: sidecarv1.VerificationVerdict(99)}}}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := validateResponse([][]byte{id}, requested, testCase.response)
			require.ErrorIs(t, err, ErrInvalidResponse)
		})
	}
}

func TestClientDeadlineAndClose(t *testing.T) {
	service := &testVerificationServer{resultHook: func(
		ctx context.Context,
		_ *sidecarv1.GetVerificationResultsRequest,
	) (*sidecarv1.GetVerificationResultsResponse, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	client, err := NewClient(startVerificationServer(t, service), TransportModeLoopback)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = client.GetVerificationResults(ctx, [][]byte{verificationID(1)})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NoError(t, client.Close())
	require.NoError(t, client.Close())
	_, err = client.GetVerificationResults(context.Background(), [][]byte{verificationID(1)})
	require.ErrorIs(t, err, ErrClientClosed)
}

func TestClientCancellation(t *testing.T) {
	service := &testVerificationServer{resultHook: func(
		ctx context.Context,
		_ *sidecarv1.GetVerificationResultsRequest,
	) (*sidecarv1.GetVerificationResultsResponse, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	client, err := NewClient(startVerificationServer(t, service), TransportModeLoopback)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.GetVerificationResults(ctx, [][]byte{verificationID(1)})
	require.ErrorIs(t, err, context.Canceled)
}

func TestClientAcceptsMaximumBatch(t *testing.T) {
	ids := make([][]byte, maxVerificationIDs)
	for i := range ids {
		id := make([]byte, verificationtypes.VerificationIDSize)
		id[0] = byte(i >> 8)
		id[1] = byte(i)
		ids[i] = id
	}
	service := &testVerificationServer{response: &sidecarv1.GetVerificationResultsResponse{}}
	client, err := NewClient(startVerificationServer(t, service), TransportModeLoopback)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	results, err := client.GetVerificationResults(context.Background(), ids)
	require.NoError(t, err)
	require.Empty(t, results)
	service.mu.Lock()
	require.Len(t, service.requests[0].VerificationIds, maxVerificationIDs)
	service.mu.Unlock()
}

func TestClientRejectsOversizedResponse(t *testing.T) {
	service := &testVerificationServer{response: &sidecarv1.GetVerificationResultsResponse{Results: []*sidecarv1.VerificationResult{{
		VerificationId: bytes.Repeat([]byte{1}, maxRPCMessageSize+1),
		Verdict:        sidecarv1.VerificationVerdict_VERIFICATION_VERDICT_VALID,
	}}}}
	client, err := NewClient(startVerificationServer(t, service), TransportModeLoopback)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	_, err = client.GetVerificationResults(context.Background(), [][]byte{verificationID(1)})
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}

func TestClientReportsUnavailableEndpoint(t *testing.T) {
	client, err := NewClient("127.0.0.1:1", TransportModeLoopback)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = client.GetVerificationResults(ctx, [][]byte{verificationID(1)})
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrInvalidResponse)
}

func TestClientConstructionIsLazy(t *testing.T) {
	client, err := NewClient("127.0.0.1:1", TransportModeLoopback)
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

func BenchmarkValidateResponse512(b *testing.B) {
	ids := make([][]byte, maxVerificationIDs)
	results := make([]*sidecarv1.VerificationResult, maxVerificationIDs)
	for i := range ids {
		id := make([]byte, verificationtypes.VerificationIDSize)
		id[0] = byte(i >> 8)
		id[1] = byte(i)
		ids[i] = id
		results[i] = &sidecarv1.VerificationResult{
			VerificationId: id, Verdict: sidecarv1.VerificationVerdict_VERIFICATION_VERDICT_VALID,
		}
	}
	_, requested, err := buildRequest(ids)
	require.NoError(b, err)
	response := &sidecarv1.GetVerificationResultsResponse{Results: results}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := validateResponse(ids, requested, response); err != nil {
			b.Fatal(err)
		}
	}
}
