package types

import (
	"context"

	"cosmossdk.io/core/address"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// StakingKeeper provides the immutable validator history and address codec used
// to materialize proof-height consensus power.
type StakingKeeper interface {
	GetHistoricalInfo(context.Context, int64) (stakingtypes.HistoricalInfo, error)
	ValidatorAddressCodec() address.Codec
}
