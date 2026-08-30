package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

// blockHeight converts the SDK height without allowing a negative value to wrap
// into the uint64 protocol domain. Protocol offsets are defined on non-negative
// absolute heights, so an unchecked conversion could turn invalid test or
// initialization input into a huge, apparently valid lifecycle height.
func blockHeight(ctx sdk.Context) (uint64, error) {
	if ctx.BlockHeight() < 0 {
		return 0, types.ErrInvalidCommitmentHeight
	}

	return uint64(ctx.BlockHeight()), nil
}
