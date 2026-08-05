package app

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpcHealthV1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

func TestAppRegistersGRPCHealthService(t *testing.T) {
	wrapperAddress := startArchiveWrapperHealthServer(
		t,
		grpcHealthV1.HealthCheckResponse_NOT_SERVING,
	)
	pulsarApp := newZeroHeightExportTestApp(t, wrapperAddress)

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	pulsarApp.RegisterGRPCServerWithSkipCheckHeader(grpcServer, false)
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///pulsar-health",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	response, err := grpcHealthV1.NewHealthClient(conn).Check(
		context.Background(),
		&grpcHealthV1.HealthCheckRequest{},
	)
	require.NoError(t, err)
	require.Equal(t, grpcHealthV1.HealthCheckResponse_SERVING, response.GetStatus())
}
