package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/server"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpcHealthV1 "google.golang.org/grpc/health/grpc_health_v1"
)

const nodeGRPCReadinessTimeout = 3 * time.Second

func nodeGRPCHealthCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "grpc",
		Short:        "Check Pulsar gRPC readiness",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return checkConfiguredNodeGRPCReady(cmd)
		},
	}
}

func checkConfiguredNodeGRPCReady(cmd *cobra.Command) error {
	serverCtx := server.GetServerContextFromCmd(cmd)
	if serverCtx == nil || serverCtx.Viper == nil {
		return errors.New("server configuration is unavailable")
	}

	appConfig, err := serverconfig.GetConfig(serverCtx.Viper)
	if err != nil {
		return fmt.Errorf("load Pulsar gRPC configuration: %w", err)
	}
	if !appConfig.GRPC.Enable {
		return errors.New("Pulsar gRPC server is disabled")
	}

	return checkNodeGRPCReady(cmd.Context(), appConfig.GRPC.Address, nodeGRPCReadinessTimeout)
}

func checkNodeGRPCReady(ctx context.Context, listenAddress string, timeout time.Duration) error {
	if timeout <= 0 {
		return errors.New("Pulsar gRPC readiness timeout must be positive")
	}

	dialAddress, err := nodeGRPCDialAddress(listenAddress)
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(
		dialAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("create Pulsar gRPC health client: %w", err)
	}
	defer func() { _ = conn.Close() }()

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	response, err := grpcHealthV1.NewHealthClient(conn).Check(
		checkCtx,
		&grpcHealthV1.HealthCheckRequest{},
	)
	if err != nil {
		return fmt.Errorf("Pulsar gRPC health check failed: %w", err)
	}
	if response.GetStatus() != grpcHealthV1.HealthCheckResponse_SERVING {
		return fmt.Errorf("Pulsar gRPC server is not serving: %s", response.GetStatus())
	}

	return nil
}

func nodeGRPCDialAddress(listenAddress string) (string, error) {
	if listenAddress != strings.TrimSpace(listenAddress) || listenAddress == "" {
		return "", fmt.Errorf("invalid Pulsar gRPC listen address %q", listenAddress)
	}

	host, port, err := net.SplitHostPort(listenAddress)
	if err != nil {
		return "", fmt.Errorf("invalid Pulsar gRPC listen address %q: %w", listenAddress, err)
	}

	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsedPort == 0 {
		return "", fmt.Errorf("invalid Pulsar gRPC port %q", port)
	}

	switch host {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	}

	return net.JoinHostPort(host, port), nil
}
