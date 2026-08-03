package keeper

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strconv"
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

type ArchiveWrapperTransportMode string

const (
	ArchiveWrapperTransportModeLoopback       ArchiveWrapperTransportMode = "loopback"
	ArchiveWrapperTransportModeTrustedNetwork ArchiveWrapperTransportMode = "trusted-network"
)

type ArchiveWrapperClient struct {
	conn         *grpc.ClientConn
	query        wrapperquery.QueryClient
	health       grpcHealthV1.HealthClient
	queryTimeout time.Duration
}

func ParseArchiveWrapperTransportMode(value string) (ArchiveWrapperTransportMode, error) {
	if value != strings.TrimSpace(value) {
		return "", errorsmod.Wrap(
			types.ErrInvalidArchiveWrapperGRPCTransportMode,
			"surrounding whitespace is not allowed",
		)
	}

	mode := ArchiveWrapperTransportMode(value)
	switch mode {
	case ArchiveWrapperTransportModeLoopback, ArchiveWrapperTransportModeTrustedNetwork:
		return mode, nil
	default:
		return "", errorsmod.Wrapf(
			types.ErrInvalidArchiveWrapperGRPCTransportMode,
			"unsupported mode %q",
			value,
		)
	}
}

func validateArchiveWrapperGRPCAddress(addr string, mode ArchiveWrapperTransportMode) error {
	switch mode {
	case ArchiveWrapperTransportModeLoopback, ArchiveWrapperTransportModeTrustedNetwork:
	default:
		return types.ErrInvalidArchiveWrapperGRPCTransportMode
	}

	if addr != strings.TrimSpace(addr) {
		return errorsmod.Wrap(
			types.ErrInvalidArchiveWrapperGRPCAddress,
			"surrounding whitespace is not allowed",
		)
	}

	if addr == "" {
		return errorsmod.Wrap(
			types.ErrInvalidArchiveWrapperGRPCAddress,
			"address is required",
		)
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

	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsedPort == 0 {
		return errorsmod.Wrapf(
			types.ErrInvalidArchiveWrapperGRPCAddress,
			"port %q must be between 1 and 65535",
			port,
		)
	}

	ip, parseErr := netip.ParseAddr(host)
	if parseErr == nil {
		if ip.Zone() != "" {
			return errorsmod.Wrapf(
				types.ErrInvalidArchiveWrapperGRPCAddress,
				"scoped IPv6 host %q is not allowed",
				host,
			)
		}

		if ip.IsUnspecified() {
			return errorsmod.Wrapf(
				types.ErrInvalidArchiveWrapperGRPCAddress,
				"wildcard host %q is not a valid client endpoint",
				host,
			)
		}

		switch mode {
		case ArchiveWrapperTransportModeLoopback:
			if !ip.IsLoopback() {
				return errorsmod.Wrapf(
					types.ErrInvalidArchiveWrapperGRPCAddress,
					"host %q is not loopback",
					host,
				)
			}
		case ArchiveWrapperTransportModeTrustedNetwork:
			if !ip.IsLoopback() && !ip.IsPrivate() {
				return errorsmod.Wrapf(
					types.ErrInvalidArchiveWrapperGRPCAddress,
					"host %q is neither loopback nor private",
					host,
				)
			}
		default:
			return types.ErrInvalidArchiveWrapperGRPCTransportMode
		}

		return nil
	}

	if mode != ArchiveWrapperTransportModeTrustedNetwork {
		return errorsmod.Wrapf(
			types.ErrInvalidArchiveWrapperGRPCAddress,
			"host %q must be a literal loopback IP address",
			host,
		)
	}

	if looksLikeIPv4Literal(host) || !isValidDNSName(host) {
		return errorsmod.Wrapf(
			types.ErrInvalidArchiveWrapperGRPCAddress,
			"host %q is not a valid DNS service name",
			host,
		)
	}

	return nil
}

func looksLikeIPv4Literal(host string) bool {
	if !strings.Contains(host, ".") {
		return false
	}

	for _, char := range host {
		if (char < '0' || char > '9') && char != '.' {
			return false
		}
	}

	return true
}

func isValidDNSName(host string) bool {
	if host == "" || len(host) > 253 || strings.HasSuffix(host, ".") {
		return false
	}

	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		for _, char := range label {
			if (char < 'a' || char > 'z') &&
				(char < 'A' || char > 'Z') &&
				(char < '0' || char > '9') &&
				char != '-' {
				return false
			}
		}
	}

	return true
}

func NewArchiveWrapperQueryClient(
	wrapperGRPCAddress string,
	transportMode ArchiveWrapperTransportMode,
) (*ArchiveWrapperClient, error) {
	if err := validateArchiveWrapperGRPCAddress(wrapperGRPCAddress, transportMode); err != nil {
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
			// Nil actions are intentionally skipped so one malformed entry does not
			// cause the whole batch to be dropped.
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
