package cmd

import (
	"context"
	"net"
	"testing"
	"time"

	"cosmossdk.io/log"
	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/node101-io/pulsar-chain/x/bridge/keeper"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpcHealthV1 "google.golang.org/grpc/health/grpc_health_v1"
)

func startWrapperHealthServer(
	t *testing.T,
	servingStatus grpcHealthV1.HealthCheckResponse_ServingStatus,
) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("query.Query", servingStatus)
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

func commandWithWrapperConfig(t *testing.T, address, mode string) *cobra.Command {
	t.Helper()

	v := viper.New()
	v.Set("bridge.wrapper_grpc_address", address)
	v.Set("bridge.wrapper_grpc_transport_mode", mode)
	serverCtx := server.NewContext(v, cmtcfg.DefaultConfig(), log.NewNopLogger())
	cmd := &cobra.Command{}
	cmd.SetContext(context.WithValue(context.Background(), server.ServerContextKey, serverCtx))

	return cmd
}

func TestCheckArchiveWrapperReady(t *testing.T) {
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
			address := startWrapperHealthServer(t, tc.status)
			err := checkArchiveWrapperReady(
				context.Background(),
				address,
				keeper.ArchiveWrapperTransportModeLoopback,
				time.Second,
			)
			if tc.wantOK {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestCheckArchiveWrapperReadyRejectsUnavailableEndpoint(t *testing.T) {
	err := checkArchiveWrapperReady(
		context.Background(),
		"127.0.0.1:1",
		keeper.ArchiveWrapperTransportModeLoopback,
		50*time.Millisecond,
	)
	require.Error(t, err)
}

func TestAddModuleInitFlagsComposesPreRunAndChecksReadiness(t *testing.T) {
	address := startWrapperHealthServer(t, grpcHealthV1.HealthCheckResponse_SERVING)
	cmd := commandWithWrapperConfig(t, address, "loopback")

	existingCalled := false
	cmd.PreRunE = func(_ *cobra.Command, _ []string) error {
		existingCalled = true
		return nil
	}

	addModuleInitFlags(cmd)
	require.NoError(t, cmd.PreRunE(cmd, nil))
	require.True(t, existingCalled)
}

func TestAddModuleInitFlagsPreservesExistingPreRunFailure(t *testing.T) {
	cmd := commandWithWrapperConfig(t, "invalid", "invalid")
	expectedErr := context.Canceled
	cmd.PreRunE = func(_ *cobra.Command, _ []string) error {
		return expectedErr
	}

	addModuleInitFlags(cmd)
	require.ErrorIs(t, cmd.PreRunE(cmd, nil), expectedErr)
}

func TestConfiguredArchiveWrapperReadinessRejectsInvalidConfig(t *testing.T) {
	cmd := commandWithWrapperConfig(t, "archive-wrapper:9095", "loopback")
	err := checkConfiguredArchiveWrapperReady(cmd)
	require.Error(t, err)
}
