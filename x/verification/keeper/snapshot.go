package keeper

import (
	"context"
	"math"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"
	comettypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

type validatorPowerEntry struct {
	address []byte
	power   int64
}

// MaterializeValidatorPowers copies the immutable staking HistoricalInfo for a
// proof height into verification's compact voting-power snapshot. The complete
// history record is validated before any writes, and the total is written last
// so it also acts as the marker for a complete snapshot.
func (k Keeper) MaterializeValidatorPowers(ctx context.Context, height uint64) error {
	exists, err := k.TotalVotingPowerByHeight.Has(ctx, height)
	if err != nil {
		return err
	}
	if exists {
		totalPower, err := k.TotalVotingPowerByHeight.Get(ctx, height)
		if err != nil {
			return err
		}
		return k.validateStoredValidatorPowers(ctx, height, totalPower)
	}

	// A power entry without the final total marker is never a valid snapshot.
	// Reject it instead of silently completing potentially corrupted state.
	orphanedPower := false
	rangeByHeight := collections.NewPrefixedPairRange[uint64, []byte](height)
	if err := k.ValidatorPowers.Walk(ctx, rangeByHeight, func(_ types.ValidatorPowerStoreKey, _ int64) (bool, error) {
		orphanedPower = true
		return true, nil
	}); err != nil {
		return err
	}
	if orphanedPower {
		return types.ErrProofStateCorrupted
	}
	if height > math.MaxInt64 {
		return errorsmod.Wrap(types.ErrProofStateCorrupted, "proof height exceeds int64")
	}

	historicalInfo, err := k.stakingKeeper.GetHistoricalInfo(ctx, int64(height))
	if err != nil {
		return err
	}
	if historicalInfo.Header.Height != int64(height) {
		return errorsmod.Wrap(types.ErrProofStateCorrupted, "historical info height mismatch")
	}
	if len(historicalInfo.Valset) == 0 {
		return types.ErrEmptyValidatorSet
	}

	entries := make([]validatorPowerEntry, 0, len(historicalInfo.Valset))
	seen := make(map[string]struct{}, len(historicalInfo.Valset))
	validatorCodec := k.stakingKeeper.ValidatorAddressCodec()
	var totalPower int64
	for _, validator := range historicalInfo.Valset {
		address, err := validatorCodec.StringToBytes(validator.GetOperator())
		if err != nil {
			return errorsmod.Wrap(types.ErrInvalidValidator, "invalid historical validator address")
		}
		key := string(address)
		if _, exists := seen[key]; exists {
			return errorsmod.Wrap(types.ErrProofStateCorrupted, "duplicate historical validator")
		}
		seen[key] = struct{}{}

		power := validator.GetConsensusPower(sdk.DefaultPowerReduction)
		if power <= 0 {
			return errorsmod.Wrap(types.ErrProofStateCorrupted, "validator has non-positive consensus power")
		}
		if power > comettypes.MaxTotalVotingPower-totalPower {
			return errorsmod.Wrap(types.ErrProofStateCorrupted, "total consensus power exceeds CometBFT limit")
		}
		totalPower += power
		entries = append(entries, validatorPowerEntry{
			address: append([]byte(nil), address...),
			power:   power,
		})
	}
	if !types.IsValidTotalVotingPower(totalPower) {
		return types.ErrProofStateCorrupted
	}

	for _, entry := range entries {
		if err := k.ValidatorPowers.Set(ctx, types.NewValidatorPowerStoreKey(height, entry.address), entry.power); err != nil {
			return err
		}
	}
	return k.TotalVotingPowerByHeight.Set(ctx, height, totalPower)
}

func (k Keeper) validateStoredValidatorPowers(ctx context.Context, height uint64, totalPower int64) error {
	if !types.IsValidTotalVotingPower(totalPower) {
		return types.ErrProofStateCorrupted
	}
	var sum int64
	count := 0
	rangeByHeight := collections.NewPrefixedPairRange[uint64, []byte](height)
	if err := k.ValidatorPowers.Walk(ctx, rangeByHeight, func(key types.ValidatorPowerStoreKey, power int64) (bool, error) {
		if len(key.K2()) == 0 || power <= 0 || power > totalPower-sum {
			return true, types.ErrProofStateCorrupted
		}
		sum += power
		count++
		return false, nil
	}); err != nil {
		return err
	}
	if count == 0 || sum != totalPower {
		return types.ErrProofStateCorrupted
	}
	return nil
}

// ValidatorPowerAtHeight returns one validator's fixed contribution and the
// total denominator for the proof height. Missing membership is an ordinary
// invalid-validator error; malformed snapshot values indicate state corruption.
func (k Keeper) ValidatorPowerAtHeight(ctx context.Context, proofHeight uint64, validator []byte) (int64, int64, error) {
	totalExists, err := k.TotalVotingPowerByHeight.Has(ctx, proofHeight)
	if err != nil {
		return 0, 0, err
	}
	if !totalExists {
		return 0, 0, types.ErrProofStateCorrupted
	}
	totalPower, err := k.TotalVotingPowerByHeight.Get(ctx, proofHeight)
	if err != nil {
		return 0, 0, err
	}
	if !types.IsValidTotalVotingPower(totalPower) {
		return 0, 0, types.ErrProofStateCorrupted
	}

	key := types.NewValidatorPowerStoreKey(proofHeight, validator)
	exists, err := k.ValidatorPowers.Has(ctx, key)
	if err != nil {
		return 0, 0, err
	}
	if !exists {
		return 0, 0, types.ErrInvalidValidator
	}
	validatorPower, err := k.ValidatorPowers.Get(ctx, key)
	if err != nil {
		return 0, 0, err
	}
	if validatorPower <= 0 || validatorPower > totalPower {
		return 0, 0, types.ErrProofStateCorrupted
	}

	return validatorPower, totalPower, nil
}

// RemoveValidatorPowers deletes a proof height's compact historical snapshot.
func (k Keeper) RemoveValidatorPowers(ctx context.Context, height uint64) error {
	keys := make([]types.ValidatorPowerStoreKey, 0)
	rangeByHeight := collections.NewPrefixedPairRange[uint64, []byte](height)
	if err := k.ValidatorPowers.Walk(ctx, rangeByHeight, func(key types.ValidatorPowerStoreKey, _ int64) (bool, error) {
		keys = append(keys, key)
		return false, nil
	}); err != nil {
		return err
	}
	for _, key := range keys {
		if err := k.ValidatorPowers.Remove(ctx, key); err != nil {
			return err
		}
	}
	return k.TotalVotingPowerByHeight.Remove(ctx, height)
}
