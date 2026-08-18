package types

import (
	"context"

	"cosmossdk.io/core/address"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type StakingKeeper interface {
	GetLastValidators(context.Context) ([]stakingtypes.Validator, error)
	ValidatorAddressCodec() address.Codec
}
