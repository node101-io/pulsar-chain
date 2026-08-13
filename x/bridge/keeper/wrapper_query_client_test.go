package keeper

import (
	"context"
	"testing"
	"time"

	wrapperactions "github.com/node101-io/archive-wrapper/actions"
	wrapperquery "github.com/node101-io/archive-wrapper/query"
	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/bridge/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubWrapperQueryClient struct {
	actionsResponse *wrapperquery.QueryGetActionsInRangeResponse
	actionsErr      error
	actionsRequest  *wrapperquery.QueryGetActionsInRangeRequest
}

func (s *stubWrapperQueryClient) GetActionsInRange(
	_ context.Context,
	req *wrapperquery.QueryGetActionsInRangeRequest,
	_ ...grpc.CallOption,
) (*wrapperquery.QueryGetActionsInRangeResponse, error) {
	s.actionsRequest = req
	return s.actionsResponse, s.actionsErr
}

func (s *stubWrapperQueryClient) GetMinaBlockHeight(
	context.Context,
	*wrapperquery.QueryGetMinaBlockHeightRequest,
	...grpc.CallOption,
) (*wrapperquery.QueryGetMinaBlockHeightResponse, error) {
	return &wrapperquery.QueryGetMinaBlockHeightResponse{}, nil
}

func TestParseArchiveWrapperTransportMode(t *testing.T) {
	for _, value := range []string{"loopback", "trusted-network"} {
		mode, err := ParseArchiveWrapperTransportMode(value)
		require.NoError(t, err)
		require.Equal(t, ArchiveWrapperTransportMode(value), mode)
	}

	for _, value := range []string{"", "trusted_network", " loopback"} {
		mode, err := ParseArchiveWrapperTransportMode(value)
		require.Empty(t, mode)
		require.ErrorIs(t, err, types.ErrInvalidArchiveWrapperGRPCTransportMode)
	}
}

func TestValidateArchiveWrapperGRPCAddress(t *testing.T) {
	testCases := []struct {
		name    string
		addr    string
		mode    ArchiveWrapperTransportMode
		wantErr bool
		modeErr bool
	}{
		{
			name:    "ipv4 loopback",
			addr:    "127.0.0.1:9095",
			mode:    ArchiveWrapperTransportModeLoopback,
			wantErr: false,
		},
		{
			name:    "ipv6 loopback",
			addr:    "[::1]:9095",
			mode:    ArchiveWrapperTransportModeLoopback,
			wantErr: false,
		},
		{
			name:    "ipv4 loopback range",
			addr:    "127.20.30.40:9095",
			mode:    ArchiveWrapperTransportModeLoopback,
			wantErr: false,
		},
		{name: "trusted dns service", addr: "archive-wrapper:9095", mode: ArchiveWrapperTransportModeTrustedNetwork},
		{name: "trusted qualified dns service", addr: "archive-wrapper-validator1.internal:9095", mode: ArchiveWrapperTransportModeTrustedNetwork},
		{name: "trusted private ipv4", addr: "192.168.1.10:9095", mode: ArchiveWrapperTransportModeTrustedNetwork},
		{name: "trusted private ipv6", addr: "[fd00::10]:9095", mode: ArchiveWrapperTransportModeTrustedNetwork},
		{
			name:    "empty address is rejected",
			addr:    "",
			mode:    ArchiveWrapperTransportModeLoopback,
			wantErr: true,
		},
		{
			name:    "surrounding whitespace is rejected",
			addr:    " 127.0.0.1:9095 ",
			mode:    ArchiveWrapperTransportModeLoopback,
			wantErr: true,
		},
		{
			name:    "localhost is rejected",
			addr:    "localhost:9095",
			mode:    ArchiveWrapperTransportModeLoopback,
			wantErr: true,
		},
		{
			name:    "wildcard is rejected",
			addr:    "0.0.0.0:9095",
			mode:    ArchiveWrapperTransportModeTrustedNetwork,
			wantErr: true,
		},
		{
			name:    "ipv6 wildcard is rejected",
			addr:    "[::]:9095",
			mode:    ArchiveWrapperTransportModeTrustedNetwork,
			wantErr: true,
		},
		{
			name:    "lan address is rejected",
			addr:    "192.168.1.10:9095",
			mode:    ArchiveWrapperTransportModeLoopback,
			wantErr: true,
		},
		{
			name:    "public address is rejected",
			addr:    "8.8.8.8:9095",
			mode:    ArchiveWrapperTransportModeTrustedNetwork,
			wantErr: true,
		},
		{
			name:    "missing host is rejected",
			addr:    ":9095",
			mode:    ArchiveWrapperTransportModeTrustedNetwork,
			wantErr: true,
		},
		{
			name:    "missing port is rejected",
			addr:    "127.0.0.1",
			mode:    ArchiveWrapperTransportModeLoopback,
			wantErr: true,
		},
		{
			name:    "zero port is rejected",
			addr:    "127.0.0.1:0",
			mode:    ArchiveWrapperTransportModeLoopback,
			wantErr: true,
		},
		{
			name:    "port above range is rejected",
			addr:    "127.0.0.1:65536",
			mode:    ArchiveWrapperTransportModeLoopback,
			wantErr: true,
		},
		{
			name:    "service name port is rejected",
			addr:    "127.0.0.1:http",
			mode:    ArchiveWrapperTransportModeLoopback,
			wantErr: true,
		},
		{
			name:    "zoned ipv6 is rejected",
			addr:    "[::1%lo]:9095",
			mode:    ArchiveWrapperTransportModeTrustedNetwork,
			wantErr: true,
		},
		{name: "url scheme is rejected", addr: "http://archive-wrapper:9095", mode: ArchiveWrapperTransportModeTrustedNetwork, wantErr: true},
		{name: "userinfo is rejected", addr: "user@archive-wrapper:9095", mode: ArchiveWrapperTransportModeTrustedNetwork, wantErr: true},
		{name: "invalid dns label is rejected", addr: "archive_wrapper:9095", mode: ArchiveWrapperTransportModeTrustedNetwork, wantErr: true},
		{name: "malformed ipv4 is rejected", addr: "127.0.0.999:9095", mode: ArchiveWrapperTransportModeTrustedNetwork, wantErr: true},
		{name: "hostname rejected for unspecified mode", addr: "archive-wrapper:9095", mode: "", wantErr: true, modeErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateArchiveWrapperGRPCAddress(tc.addr, tc.mode)
			if tc.wantErr {
				if tc.modeErr {
					require.ErrorIs(t, err, types.ErrInvalidArchiveWrapperGRPCTransportMode)
					return
				}
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
		mode ArchiveWrapperTransportMode
	}{
		{name: "empty", addr: "", mode: ArchiveWrapperTransportModeLoopback},
		{name: "malformed", addr: "127.0.0.1", mode: ArchiveWrapperTransportModeLoopback},
		{name: "non-loopback", addr: "192.168.1.10:9095", mode: ArchiveWrapperTransportModeLoopback},
		{name: "invalid mode", addr: "127.0.0.1:9095", mode: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewArchiveWrapperQueryClient(tc.addr, tc.mode)
			require.Nil(t, client)
			if tc.mode == "" {
				require.ErrorIs(t, err, types.ErrInvalidArchiveWrapperGRPCTransportMode)
				return
			}
			require.ErrorIs(t, err, types.ErrInvalidArchiveWrapperGRPCAddress)
		})
	}
}

func TestNewArchiveWrapperQueryClientAcceptsSupportedEndpoints(t *testing.T) {
	for _, tc := range []struct {
		addr string
		mode ArchiveWrapperTransportMode
	}{
		{addr: "127.0.0.1:9095", mode: ArchiveWrapperTransportModeLoopback},
		{addr: "archive-wrapper:9095", mode: ArchiveWrapperTransportModeTrustedNetwork},
	} {
		client, err := NewArchiveWrapperQueryClient(tc.addr, tc.mode)
		require.NoError(t, err)
		require.NotNil(t, client)
		require.NoError(t, client.Close())
	}
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

func TestArchiveWrapperClientGetActionsInRangeMapsNewActionFields(t *testing.T) {
	xCoordinate := []byte{1, 2, 3}
	queryClient := &stubWrapperQueryClient{
		actionsResponse: &wrapperquery.QueryGetActionsInRangeResponse{
			Actions: []*wrapperactions.Action{
				nil,
				{
					BlockHeight: 11,
					XCoordinate: xCoordinate,
					IsOdd:       true,
					ActionType:  wrapperactions.ActionType_DEPOSIT,
					Amount:      7,
				},
			},
		},
	}
	client := &ArchiveWrapperClient{
		conn:         &grpc.ClientConn{},
		query:        queryClient,
		queryTimeout: time.Second,
	}

	actions, err := client.GetActionsInRange(context.Background(), 10, 12)
	require.NoError(t, err)
	require.Equal(t, &wrapperquery.QueryGetActionsInRangeRequest{
		StartBlockHeight: 11,
		EndBlockHeight:   12,
	}, queryClient.actionsRequest)
	require.Equal(t, []types.Action{
		{
			BlockHeight: 11,
			XCoordinate: xCoordinate,
			IsOdd:       true,
			ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
			Amount:      7,
		},
	}, actions)
}
