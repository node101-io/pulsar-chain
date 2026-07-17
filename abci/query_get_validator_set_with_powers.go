package abci

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *ABCIHandler) GetValidatorSetWithPowers(ctx context.Context, req *QueryGetValidatorSetWithPowersRequest) (*QueryGetValidatorSetWithPowersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	return &QueryGetValidatorSetWithPowersResponse{}, nil
}
