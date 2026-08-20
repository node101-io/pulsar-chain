package sidecar

import (
	"context"
)

// ResultValue is the chain client's terminal result domain. Unspecified is a
// validation sentinel and never means invalid. The chain deliberately reduces
// richer sidecar lifecycle states to the only two values that can be committed.
type ResultValue uint8

const (
	// ResultUnspecified is rejected by the validator-local builder.
	ResultUnspecified ResultValue = iota
	// ResultInvalid is a completed negative cryptographic verdict.
	ResultInvalid
	// ResultValid is a completed positive cryptographic verdict.
	ResultValid
)

// VerificationResult associates one requested verification ID with a terminal result.
type VerificationResult struct {
	VerificationID []byte
	Result         ResultValue
}

// Provider returns only the completed subset of a requested verification-ID batch.
// Missing IDs remain pending and must not be converted to invalid votes.
// The provider does not receive block heights or build commitments; mapping
// results to ProofKey and protocol windows remains validator-node logic.
type Provider interface {
	GetVerificationResults(context.Context, [][]byte) ([]VerificationResult, error)
}

// ProviderFunc adapts a function to Provider, primarily for tests and embedding.
type ProviderFunc func(context.Context, [][]byte) ([]VerificationResult, error)

// GetVerificationResults calls the adapted provider function.
func (f ProviderFunc) GetVerificationResults(ctx context.Context, verificationIDs [][]byte) ([]VerificationResult, error) {
	return f(ctx, verificationIDs)
}

// DisabledProvider represents an intentionally disabled optional verifier. It
// behaves like a sidecar with no terminal results, allowing the rest of the
// consensus path to run unchanged.
type DisabledProvider struct{}

// GetVerificationResults returns no terminal results without contacting a sidecar.
func (DisabledProvider) GetVerificationResults(context.Context, [][]byte) ([]VerificationResult, error) {
	return nil, nil
}
