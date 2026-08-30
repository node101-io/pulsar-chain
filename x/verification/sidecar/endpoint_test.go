package sidecar

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTransportMode(t *testing.T) {
	for _, mode := range []TransportMode{TransportModeLoopback, TransportModeTrustedNetwork} {
		parsed, err := ParseTransportMode(string(mode))
		require.NoError(t, err)
		require.Equal(t, mode, parsed)
	}
	for _, value := range []string{"", " loopback", "loopback ", "public"} {
		_, err := ParseTransportMode(value)
		require.ErrorIs(t, err, ErrInvalidTransport)
	}
}

func TestValidateGRPCAddress(t *testing.T) {
	testCases := []struct {
		name    string
		address string
		mode    TransportMode
		valid   bool
	}{
		{name: "IPv4 loopback", address: "127.0.0.1:50051", mode: TransportModeLoopback, valid: true},
		{name: "IPv6 loopback", address: "[::1]:50051", mode: TransportModeLoopback, valid: true},
		{name: "private trusted", address: "10.0.0.2:50051", mode: TransportModeTrustedNetwork, valid: true},
		{name: "DNS trusted", address: "pulsar-verifier:50051", mode: TransportModeTrustedNetwork, valid: true},
		{name: "DNS loopback", address: "localhost:50051", mode: TransportModeLoopback},
		{name: "private loopback", address: "192.168.1.2:50051", mode: TransportModeLoopback},
		{name: "public trusted", address: "8.8.8.8:50051", mode: TransportModeTrustedNetwork},
		{name: "wildcard", address: "0.0.0.0:50051", mode: TransportModeTrustedNetwork},
		{name: "scoped IPv6", address: "[fe80::1%eth0]:50051", mode: TransportModeTrustedNetwork},
		{name: "bad IPv4", address: "999.1.1.1:50051", mode: TransportModeTrustedNetwork},
		{name: "bad DNS", address: "bad_name:50051", mode: TransportModeTrustedNetwork},
		{name: "missing port", address: "127.0.0.1", mode: TransportModeLoopback},
		{name: "zero port", address: "127.0.0.1:0", mode: TransportModeLoopback},
		{name: "whitespace", address: " 127.0.0.1:50051", mode: TransportModeLoopback},
		{name: "invalid mode", address: "127.0.0.1:50051", mode: ""},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateGRPCAddress(testCase.address, testCase.mode)
			if testCase.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}
