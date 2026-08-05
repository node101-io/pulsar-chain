package cmd

import (
	"context"
	"net"
	"testing"
	"time"

	"cosmossdk.io/log"
	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpcHealthV1 "google.golang.org/grpc/health/grpc_health_v1"
)

func startNodeHealthServer(
	t *testing.T,
	servingStatus grpcHealthV1.HealthCheckResponse_ServingStatus,
) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", servingStatus)
	grpcHealthV1.RegisterHealthServer(grpcServer, healthServer)

	go func() {
		_ = grpcServer.Serve(listener)
	}()

	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})

	return listener.Addr().String()
}

func commandWithNodeGRPCConfig(t *testing.T, address string, enabled bool) *cobra.Command {
	t.Helper()

	v := viper.New()
	v.Set("grpc.address", address)
	v.Set("grpc.enable", enabled)
	serverCtx := server.NewContext(v, cmtcfg.DefaultConfig(), log.NewNopLogger())
	cmd := &cobra.Command{}
	cmd.SetContext(context.WithValue(context.Background(), server.ServerContextKey, serverCtx))

	return cmd
}

func TestCheckNodeGRPCReady(t *testing.T) {
	testCases := []struct {
		name   string
		status grpcHealthV1.HealthCheckResponse_ServingStatus
		wantOK bool
	}{
		{name: "serving", status: grpcHealthV1.HealthCheckResponse_SERVING, wantOK: true},
		{name: "not serving", status: grpcHealthV1.HealthCheckResponse_NOT_SERVING},
		{name: "unknown", status: grpcHealthV1.HealthCheckResponse_UNKNOWN},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			address := startNodeHealthServer(t, tc.status)
			err := checkNodeGRPCReady(context.Background(), address, time.Second)
			if tc.wantOK {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestCheckNodeGRPCReadyRejectsUnavailableEndpoint(t *testing.T) {
	err := checkNodeGRPCReady(context.Background(), "127.0.0.1:1", 50*time.Millisecond)
	require.Error(t, err)
}

func TestCheckConfiguredNodeGRPCReady(t *testing.T) {
	address := startNodeHealthServer(t, grpcHealthV1.HealthCheckResponse_SERVING)
	cmd := commandWithNodeGRPCConfig(t, address, true)
	require.NoError(t, checkConfiguredNodeGRPCReady(cmd))
}

func TestCheckConfiguredNodeGRPCReadyRejectsDisabledServer(t *testing.T) {
	cmd := commandWithNodeGRPCConfig(t, "127.0.0.1:9090", false)
	require.ErrorContains(t, checkConfiguredNodeGRPCReady(cmd), "disabled")
}

func TestCheckConfiguredNodeGRPCListenerAvailable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	cmd := commandWithNodeGRPCConfig(t, address, true)
	require.NoError(t, checkConfiguredNodeGRPCListenerAvailable(cmd))
}

func TestCheckConfiguredNodeGRPCListenerAvailableRejectsOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	cmd := commandWithNodeGRPCConfig(t, listener.Addr().String(), true)
	err = checkConfiguredNodeGRPCListenerAvailable(cmd)
	require.ErrorContains(t, err, "unavailable")
}

func TestNodeGRPCDialAddress(t *testing.T) {
	testCases := []struct {
		name    string
		address string
		want    string
		wantErr bool
	}{
		{name: "IPv4 wildcard", address: "0.0.0.0:9090", want: "127.0.0.1:9090"},
		{name: "empty wildcard", address: ":9090", want: "127.0.0.1:9090"},
		{name: "IPv6 wildcard", address: "[::]:9090", want: "[::1]:9090"},
		{name: "hostname", address: "localhost:9090", want: "localhost:9090"},
		{name: "missing port", address: "127.0.0.1", wantErr: true},
		{name: "zero port", address: "127.0.0.1:0", wantErr: true},
		{name: "named port", address: "127.0.0.1:http", wantErr: true},
		{name: "surrounding whitespace", address: " 127.0.0.1:9090", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := nodeGRPCDialAddress(tc.address)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
