package keeper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/bridge/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
			name:    "ipv4 loopback range",
			addr:    "127.20.30.40:9095",
			wantErr: false,
		},
		{
			name:    "empty address is rejected",
			addr:    "",
			wantErr: true,
		},
		{
			name:    "surrounding whitespace is rejected",
			addr:    " 127.0.0.1:9095 ",
			wantErr: true,
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
			name:    "ipv6 wildcard is rejected",
			addr:    "[::]:9095",
			wantErr: true,
		},
		{
			name:    "lan address is rejected",
			addr:    "192.168.1.10:9095",
			wantErr: true,
		},
		{
			name:    "public address is rejected",
			addr:    "8.8.8.8:9095",
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
		{
			name:    "zero port is rejected",
			addr:    "127.0.0.1:0",
			wantErr: true,
		},
		{
			name:    "port above range is rejected",
			addr:    "127.0.0.1:65536",
			wantErr: true,
		},
		{
			name:    "service name port is rejected",
			addr:    "127.0.0.1:http",
			wantErr: true,
		},
		{
			name:    "zoned ipv6 is rejected",
			addr:    "[::1%lo]:9095",
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

func TestNewArchiveWrapperQueryClientRejectsInvalidAddress(t *testing.T) {
	testCases := []struct {
		name string
		addr string
	}{
		{name: "empty", addr: ""},
		{name: "malformed", addr: "127.0.0.1"},
		{name: "non-loopback", addr: "192.168.1.10:9095"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewArchiveWrapperQueryClient(tc.addr)
			require.Nil(t, client)
			require.ErrorIs(t, err, types.ErrInvalidArchiveWrapperGRPCAddress)
		})
	}
}

func TestNewArchiveWrapperQueryClientNormalizesAddress(t *testing.T) {
	client, err := NewArchiveWrapperQueryClient(" 127.0.0.1:9095 ")
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NoError(t, client.Close())
}

func TestArchiveWrapperClientMethodsRejectUnconfiguredClient(t *testing.T) {
	var nilClient *ArchiveWrapperClient

	_, err := nilClient.GetMinaBlockHeight(context.Background())
	require.ErrorIs(t, err, types.ErrArchiveWrapperQueryClientNotConfigured)

	_, err = nilClient.GetActionsInRange(context.Background(), 10, 11)
	require.ErrorIs(t, err, types.ErrArchiveWrapperQueryClientNotConfigured)

	err = nilClient.CheckReady(context.Background())
	require.ErrorIs(t, err, types.ErrArchiveWrapperQueryClientNotConfigured)

	zeroClient := &ArchiveWrapperClient{}

	_, err = zeroClient.GetMinaBlockHeight(context.Background())
	require.ErrorIs(t, err, types.ErrArchiveWrapperQueryClientNotConfigured)

	_, err = zeroClient.GetActionsInRange(context.Background(), 10, 11)
	require.ErrorIs(t, err, types.ErrArchiveWrapperQueryClientNotConfigured)

	err = zeroClient.CheckReady(context.Background())
	require.ErrorIs(t, err, types.ErrArchiveWrapperQueryClientNotConfigured)
}

func TestMapArchiveWrapperQueryError(t *testing.T) {
	testCases := []struct {
		name    string
		err     error
		wantErr error
	}{
		{
			name:    "grpc deadline exceeded",
			err:     status.Error(codes.DeadlineExceeded, "deadline"),
			wantErr: types.ErrArchiveWrapperQueryTimeout,
		},
		{
			name:    "grpc cancelled",
			err:     status.Error(codes.Canceled, "cancelled"),
			wantErr: types.ErrArchiveWrapperQueryCancelled,
		},
		{
			name:    "context deadline exceeded",
			err:     context.DeadlineExceeded,
			wantErr: types.ErrArchiveWrapperQueryTimeout,
		},
		{
			name:    "context cancelled",
			err:     context.Canceled,
			wantErr: types.ErrArchiveWrapperQueryCancelled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, mapArchiveWrapperQueryError(tc.err), tc.wantErr)
		})
	}

	originalErr := status.Error(codes.FailedPrecondition, "earliest indexed height")
	require.Equal(t, originalErr, mapArchiveWrapperQueryError(originalErr))
}
