package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func blockHeight(ctx sdk.Context) (uint64, error) {
	if ctx.BlockHeight() < 0 {
		return 0, types.ErrInvalidCommitmentHeight
	}

	return uint64(ctx.BlockHeight()), nil
}
