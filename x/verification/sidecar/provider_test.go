package sidecar

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderFuncForwardsVerificationIDs(t *testing.T) {
	want := [][]byte{{1}, {2}}
	provider := ProviderFunc(func(_ context.Context, hashes [][]byte) ([]VerificationResult, error) {
		require.Equal(t, want, hashes)
		return []VerificationResult{{VerificationID: []byte{1}, Result: ResultValid}}, nil
	})

	got, err := provider.GetVerificationResults(context.Background(), want)
	require.NoError(t, err)
	require.Equal(t, []VerificationResult{{VerificationID: []byte{1}, Result: ResultValid}}, got)
}

func TestDisabledProviderReturnsNoCompletedResults(t *testing.T) {
	got, err := (DisabledProvider{}).GetVerificationResults(context.Background(), [][]byte{{1}})
	require.NoError(t, err)
	require.Empty(t, got)
}
