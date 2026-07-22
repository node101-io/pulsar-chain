package keeper

import (
	"context"
	"strings"

	wrapperquery "github.com/node101-io/archive-wrapper/query"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NewArchiveWrapperQueryClient constructs the wrapper gRPC client once at app
// startup and lets the keeper reuse it for requests.
func NewArchiveWrapperQueryClient(wrapperGRPCAddress string) (wrapperquery.QueryClient, error) {
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

	return wrapperquery.NewQueryClient(conn), nil
}

func (k Keeper) getWrapperMinaBlockHeight(ctx context.Context) (int64, error) {
	if k.archiveWrapperQueryClient == nil {
		return 0, types.ErrArchiveWrapperQueryClientNotConfigured
	}

	resp, err := k.archiveWrapperQueryClient.GetMinaBlockHeight(
		ctx,
		&wrapperquery.QueryGetMinaBlockHeightRequest{},
	)
	if err != nil {
		return 0, err
	}

	return resp.BlockHeight, nil
}

func (k Keeper) getWrapperActionsInRange(
	ctx context.Context,
	latestFetchedMinaHeight int64,
	targetMinaHeight int64,
) ([]types.Action, error) {
	if k.archiveWrapperQueryClient == nil {
		return nil, types.ErrArchiveWrapperQueryClientNotConfigured
	}

	startBlockHeight := latestFetchedMinaHeight + 1
	if startBlockHeight <= 0 {
		startBlockHeight = 1
	}

	if targetMinaHeight < startBlockHeight {
		return []types.Action{}, nil
	}

	resp, err := k.archiveWrapperQueryClient.GetActionsInRange(
		ctx,
		&wrapperquery.QueryGetActionsInRangeRequest{
			StartBlockHeight: startBlockHeight,
			EndBlockHeight:   targetMinaHeight,
		},
	)
	if err != nil {
		return nil, err
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
