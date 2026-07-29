package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

func TestValidateLoopbackGRPCAddress(t *testing.T) {
	testCases := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{
			name:    "ipv4 loopback",
			addr:    "127.0.0.1:9095",
			wantErr: false,
		},
		{
			name:    "ipv6 loopback",
			addr:    "[::1]:9095",
			wantErr: false,
		},
		{
			name:    "localhost is rejected",
			addr:    "localhost:9095",
			wantErr: true,
		},
		{
			name:    "wildcard is rejected",
			addr:    "0.0.0.0:9095",
			wantErr: true,
		},
		{
			name:    "lan address is rejected",
			addr:    "192.168.1.10:9095",
			wantErr: true,
		},
		{
			name:    "missing host is rejected",
			addr:    ":9095",
			wantErr: true,
		},
		{
			name:    "missing port is rejected",
			addr:    "127.0.0.1",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLoopbackGRPCAddress(tc.addr)
			if tc.wantErr {
				require.ErrorIs(t, err, types.ErrInvalidArchiveWrapperGRPCAddress)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestNewArchiveWrapperQueryClientRejectsNonLoopbackAddress(t *testing.T) {
	client, err := NewArchiveWrapperQueryClient("192.168.1.10:9095")
	require.Nil(t, client)
	require.ErrorIs(t, err, types.ErrInvalidArchiveWrapperGRPCAddress)
}
