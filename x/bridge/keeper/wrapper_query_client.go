package keeper

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	errorsmod "cosmossdk.io/errors"
	wrapperquery "github.com/node101-io/archive-wrapper/query"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	grpcHealthV1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const (
	defaultWrapperQueryTimeout  = 5 * time.Second
	wrapperQueryGRPCServiceName = "query.Query"
)

type ArchiveWrapperQueryClient interface {
	GetMinaBlockHeight(ctx context.Context) (int64, error)
	GetActionsInRange(ctx context.Context, latestFetchedMinaHeight, targetMinaHeight int64) ([]types.Action, error)
}

type ArchiveWrapperClient struct {
	conn         *grpc.ClientConn
	query        wrapperquery.QueryClient
	health       grpcHealthV1.HealthClient
	queryTimeout time.Duration
}

func validateLoopbackGRPCAddress(addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return types.ErrInvalidArchiveWrapperGRPCAddress
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return errorsmod.Wrapf(
			types.ErrInvalidArchiveWrapperGRPCAddress,
			"expected host:port, got %q: %v",
			addr,
			err,
		)
	}

	if strings.TrimSpace(port) == "" {
		return errorsmod.Wrapf(
			types.ErrInvalidArchiveWrapperGRPCAddress,
			"missing port in %q",
			addr,
		)
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return errorsmod.Wrapf(
			types.ErrInvalidArchiveWrapperGRPCAddress,
			"host %q is not a literal IP; only 127.0.0.1 or ::1 are allowed",
			host,
		)
	}

	if !ip.IsLoopback() {
		return errorsmod.Wrapf(
			types.ErrInvalidArchiveWrapperGRPCAddress,
			"host %q is not loopback; only 127.0.0.1 or ::1 are allowed",
			host,
		)
	}

	return nil
}

// Archive-wrapper and Pulsar must run on the same machine.
// Hence, wrapperGRPCAddress must be a loopback host:port address (for example 127.0.0.1:9095 or [::1]:9095).
func NewArchiveWrapperQueryClient(wrapperGRPCAddress string) (*ArchiveWrapperClient, error) {
	wrapperGRPCAddress = strings.TrimSpace(wrapperGRPCAddress)
	if wrapperGRPCAddress == "" {
		return nil, nil
	}

	if err := validateLoopbackGRPCAddress(wrapperGRPCAddress); err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(
		wrapperGRPCAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &ArchiveWrapperClient{
		conn:         conn,
		query:        wrapperquery.NewQueryClient(conn),
		health:       grpcHealthV1.NewHealthClient(conn),
		queryTimeout: defaultWrapperQueryTimeout,
	}, nil
}

func (c *ArchiveWrapperClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *ArchiveWrapperClient) CheckReady(ctx context.Context) error {
	if c == nil || c.conn == nil || c.query == nil || c.health == nil {
		return types.ErrArchiveWrapperQueryClientNotConfigured
	}

	resp, err := c.health.Check(
		ctx,
		&grpcHealthV1.HealthCheckRequest{
			Service: wrapperQueryGRPCServiceName,
		},
	)
	if err != nil {
		return mapArchiveWrapperQueryError(err)
	}

	if resp.GetStatus() != grpcHealthV1.HealthCheckResponse_SERVING {
		return errorsmod.Wrapf(
			types.ErrArchiveWrapperNotReady,
			"service %q reported health status %s",
			wrapperQueryGRPCServiceName,
			resp.GetStatus().String(),
		)
	}

	return nil
}

func (c *ArchiveWrapperClient) GetMinaBlockHeight(ctx context.Context) (int64, error) {
	if c == nil || c.conn == nil || c.query == nil {
		return 0, types.ErrArchiveWrapperQueryClientNotConfigured
	}

	rpcCtx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()

	resp, err := c.query.GetMinaBlockHeight(
		rpcCtx,
		&wrapperquery.QueryGetMinaBlockHeightRequest{},
	)
	if err != nil {
		return 0, mapArchiveWrapperQueryError(err)
	}

	return resp.BlockHeight, nil
}

func (c *ArchiveWrapperClient) GetActionsInRange(
	ctx context.Context,
	latestFetchedMinaHeight int64,
	targetMinaHeight int64,
) ([]types.Action, error) {
	if c == nil || c.conn == nil || c.query == nil {
		return nil, types.ErrArchiveWrapperQueryClientNotConfigured
	}

	startBlockHeight := latestFetchedMinaHeight + 1
	if startBlockHeight <= 0 {
		return nil, types.ErrInvalidMinaBlockRange
	}

	if targetMinaHeight < startBlockHeight {
		return nil, types.ErrInvalidMinaBlockRange
	}

	rpcCtx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()

	resp, err := c.query.GetActionsInRange(
		rpcCtx,
		&wrapperquery.QueryGetActionsInRangeRequest{
			StartBlockHeight: startBlockHeight,
			EndBlockHeight:   targetMinaHeight,
		},
	)
	if err != nil {
		return nil, mapArchiveWrapperQueryError(err)
	}

	actions := make([]types.Action, 0, len(resp.Actions))
	for _, act := range resp.Actions {
		if act == nil {
			continue
		}

		actions = append(actions, types.Action{
			BlockHeight: act.BlockHeight,
			FeePayer:    act.FeePayer,
			ActionType:  types.ActionType(act.ActionType),
			Amount:      act.Amount,
		})
	}

	return actions, nil
}

func mapArchiveWrapperQueryError(err error) error {
	if err == nil {
		return nil
	}

	switch status.Code(err) {
	case codes.DeadlineExceeded:
		return types.ErrArchiveWrapperQueryTimeout
	case codes.Canceled:
		return types.ErrArchiveWrapperQueryCancelled
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return types.ErrArchiveWrapperQueryTimeout
	}
	if errors.Is(err, context.Canceled) {
		return types.ErrArchiveWrapperQueryCancelled
	}

	return err
}
