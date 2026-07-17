package keeper

import (
	"context"

	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (q queryServer) GetHistoricalKeyregistry(ctx context.Context, req *types.QueryGetHistoricalKeyregistryRequest) (*types.QueryGetHistoricalKeyregistryResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	// TODO: Process the query

	return &types.QueryGetHistoricalKeyregistryResponse{}, nil
}
