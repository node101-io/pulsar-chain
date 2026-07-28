package keeper

import (
	"context"
	"errors"
	"strings"
	"time"

	wrapperquery "github.com/node101-io/archive-wrapper/query"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const defaultWrapperQueryTimeout = 5 * time.Second

type ArchiveWrapperQueryClient interface {
	GetMinaBlockHeight(ctx context.Context) (int64, error)
	GetActionsInRange(ctx context.Context, latestFetchedMinaHeight, targetMinaHeight int64) ([]types.Action, error)
}

type ArchiveWrapperClient struct {
	conn         *grpc.ClientConn
	query        wrapperquery.QueryClient
	queryTimeout time.Duration
}

func NewArchiveWrapperQueryClient(wrapperGRPCAddress string) (*ArchiveWrapperClient, error) {
	wrapperGRPCAddress = strings.TrimSpace(wrapperGRPCAddress)
	if wrapperGRPCAddress == "" {
		return nil, nil
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
		queryTimeout: defaultWrapperQueryTimeout,
	}, nil
}

func (c *ArchiveWrapperClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// TODO: implement a Health endpoint to archive-wrapper to replace this mock one.
func (c *ArchiveWrapperClient) CheckReady(ctx context.Context) error {
	if c == nil || c.conn == nil || c.query == nil {
		return types.ErrArchiveWrapperQueryClientNotConfigured
	}

	_, err := c.query.GetMinaBlockHeight(
		ctx,
		&wrapperquery.QueryGetMinaBlockHeightRequest{},
	)
	if err != nil {
		return mapArchiveWrapperQueryError(err)
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
