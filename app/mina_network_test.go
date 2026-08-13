package app

import (
	"testing"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	"github.com/stretchr/testify/require"
)

func TestNormalizeMinaNetworkID(t *testing.T) {
	for _, tc := range []struct {
		name      string
		input     string
		expected  mina.NetworkID
		wantError bool
	}{
		{name: "devnet", input: "devnet", expected: mina.TestNet},
		{name: "testnet", input: "testnet", expected: mina.TestNet},
		{name: "mainnet", input: "mainnet", expected: mina.MainNet},
		{name: "empty", input: "", wantError: true},
		{name: "unknown", input: "berkeley", wantError: true},
		{name: "surrounding whitespace", input: " testnet ", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := normalizeMinaNetworkID(tc.input)
			if tc.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, actual)
		})
	}
}
