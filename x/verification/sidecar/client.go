package sidecar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	sidecarv1 "github.com/node101-io/pulsar-chain/x/verification/sidecar/api/v1"
	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	maxProofHashes    = verificationtypes.MaxVoteIndexExclusive * 2
	maxRPCMessageSize = 256 << 10
)

var (
	// ErrInvalidRequest reports a malformed or oversized result request.
	ErrInvalidRequest = errors.New("invalid verification sidecar request")
	// ErrInvalidResponse reports a response that violates the result-only contract.
	ErrInvalidResponse = errors.New("invalid verification sidecar response")
	// ErrClientClosed reports use after the owned connection has been closed.
	ErrClientClosed = errors.New("verification sidecar client is closed")
)

var _ Provider = (*Client)(nil)

// Client is a persistent result-only gRPC provider for the validator builder.
// It deliberately does not expose or call the operator-only status RPC. QUEUED,
// VERIFYING, FAILED, and UNAVAILABLE are useful for operators but are not inputs
// to consensus decisions.
type Client struct {
	mu     sync.RWMutex
	conn   *grpc.ClientConn
	client sidecarv1.VerificationServiceClient
}

// NewClient validates an endpoint and creates a lazy persistent gRPC client.
// grpc.NewClient does not require the sidecar to be online during node startup.
// Reusing one connection avoids a dial and handshake in every ExtendVote call,
// while gRPC handles reconnects after a sidecar restart.
func NewClient(address string, mode TransportMode) (*Client, error) {
	if err := validateGRPCAddress(address, mode); err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(maxRPCMessageSize),
			grpc.MaxCallRecvMsgSize(maxRPCMessageSize),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create verification sidecar client: %w", err)
	}
	return &Client{conn: conn, client: sidecarv1.NewVerificationServiceClient(conn)}, nil
}

// GetVerificationResults returns the completed subset of requested proof hashes.
// The complete response is validated before any item reaches the builder. If
// one entry is malformed, no vote from that response is used; the next protocol
// opportunity can retry the still-uncommitted hashes.
func (c *Client) GetVerificationResults(
	ctx context.Context,
	proofHashes [][]byte,
) ([]VerificationResult, error) {
	request, requested, err := buildRequest(proofHashes)
	if err != nil || len(proofHashes) == 0 {
		return nil, err
	}
	if c == nil {
		return nil, ErrClientClosed
	}
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()
	if client == nil {
		return nil, ErrClientClosed
	}

	response, err := client.GetVerificationResults(ctx, request)
	if err != nil {
		return nil, mapRPCError(err)
	}
	return validateResponse(proofHashes, requested, response)
}

func mapRPCError(err error) error {
	switch status.Code(err) {
	case codes.DeadlineExceeded:
		return fmt.Errorf("get verification results: %w", context.DeadlineExceeded)
	case codes.Canceled:
		return fmt.Errorf("get verification results: %w", context.Canceled)
	default:
		return fmt.Errorf("get verification results: %w", err)
	}
}

// Close releases the underlying persistent gRPC connection.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.client = nil
	return err
}

func buildRequest(
	proofHashes [][]byte,
) (*sidecarv1.GetVerificationResultsRequest, map[string]struct{}, error) {
	if len(proofHashes) > maxProofHashes {
		return nil, nil, fmt.Errorf("%w: batch exceeds %d hashes", ErrInvalidRequest, maxProofHashes)
	}
	// Clone every hash into the protobuf request so caller mutation cannot race
	// with asynchronous gRPC serialization. The 512-hash bound covers two full
	// 256-proof blocks, exactly matching one commitment's left and right leaves.
	requested := make(map[string]struct{}, len(proofHashes))
	cloned := make([][]byte, len(proofHashes))
	for i, hash := range proofHashes {
		if len(hash) != verificationtypes.ProofHashSize {
			return nil, nil, fmt.Errorf("%w: proof hash %d must be %d bytes", ErrInvalidRequest, i, verificationtypes.ProofHashSize)
		}
		key := string(hash)
		if _, duplicate := requested[key]; duplicate {
			return nil, nil, fmt.Errorf("%w: duplicate proof hash", ErrInvalidRequest)
		}
		requested[key] = struct{}{}
		cloned[i] = bytes.Clone(hash)
	}
	return &sidecarv1.GetVerificationResultsRequest{ProofHashes: cloned}, requested, nil
}

func validateResponse(
	proofHashes [][]byte,
	requested map[string]struct{},
	response *sidecarv1.GetVerificationResultsResponse,
) ([]VerificationResult, error) {
	if response == nil {
		return nil, fmt.Errorf("%w: nil response", ErrInvalidResponse)
	}
	if len(response.Results) > len(requested) {
		return nil, fmt.Errorf("%w: too many results", ErrInvalidResponse)
	}
	// Build a hash map because the service is free to return its terminal subset
	// in any order. Every returned hash must be unique and come from this request,
	// which prevents a sidecar bug from voting on unrelated chain state.
	completed := make(map[string]ResultValue, len(response.Results))
	for _, result := range response.Results {
		if result == nil || len(result.ProofHash) != verificationtypes.ProofHashSize {
			return nil, fmt.Errorf("%w: malformed proof hash", ErrInvalidResponse)
		}
		key := string(result.ProofHash)
		if _, ok := requested[key]; !ok {
			return nil, fmt.Errorf("%w: unrequested proof hash", ErrInvalidResponse)
		}
		if _, duplicate := completed[key]; duplicate {
			return nil, fmt.Errorf("%w: duplicate proof hash", ErrInvalidResponse)
		}
		var value ResultValue
		switch result.Verdict {
		case sidecarv1.VerificationVerdict_VERIFICATION_VERDICT_VALID:
			value = ResultValid
		case sidecarv1.VerificationVerdict_VERIFICATION_VERDICT_INVALID:
			value = ResultInvalid
		default:
			return nil, fmt.Errorf("%w: unsupported verdict %d", ErrInvalidResponse, result.Verdict)
		}
		completed[key] = value
	}

	// Normalize to request order for stable downstream behavior even though the
	// wire contract is unordered. The builder still sorts by ProofKey before
	// hashing, but stable client output simplifies callers and tests.
	results := make([]VerificationResult, 0, len(completed))
	for _, hash := range proofHashes {
		if result, ok := completed[string(hash)]; ok {
			results = append(results, VerificationResult{ProofHash: bytes.Clone(hash), Result: result})
		}
	}
	return results, nil
}
