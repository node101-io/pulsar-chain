package sidecar

import (
	"context"
)

type ResultValue uint8

const (
	ResultUnspecified ResultValue = iota
	ResultInvalid
	ResultValid
)

type VerificationResult struct {
	ProofHash []byte
	Result    ResultValue
}

type Provider interface {
	GetVerificationResults(context.Context, [][]byte) ([]VerificationResult, error)
}

type ProviderFunc func(context.Context, [][]byte) ([]VerificationResult, error)

func (f ProviderFunc) GetVerificationResults(ctx context.Context, proofHashes [][]byte) ([]VerificationResult, error) {
	return f(ctx, proofHashes)
}

type DisabledProvider struct{}

func (DisabledProvider) GetVerificationResults(context.Context, [][]byte) ([]VerificationResult, error) {
	return nil, nil
}
