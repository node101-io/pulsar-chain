package types

import (
	"context"

	"cosmossdk.io/core/address"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// StakingKeeper provides the validator set and address codec required to take
// immutable eligibility snapshots at proof submission heights.
type StakingKeeper interface {
	GetLastValidators(context.Context) ([]stakingtypes.Validator, error)
	ValidatorAddressCodec() address.Codec
}
