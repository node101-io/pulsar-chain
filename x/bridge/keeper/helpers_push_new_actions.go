package keeper

import (
	"context"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
	keyregistryTypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

const (
	UNSPECIFIED = iota
	DEPOSIT
	WITHDRAW
)

func (k *Keeper) isValid(ctx context.Context, act *FetchedAction) (bool, error) {
	if k.keyRegistryKeeper == nil {
		return false, types.ErrKeyRegistryKeeperNotConfigured
	}
	switch act.actionType {
	case DEPOSIT:
		return k.isValidDeposit(ctx, act)
	case WITHDRAW:
		return k.isValidWithdrawal(ctx, act)
	default:
		return false, types.ErrUnspecified
	}
}

func (k *Keeper) isValidDeposit(ctx context.Context, act *FetchedAction) (bool, error) {

	exists, err := k.keyRegistryKeeper.UserMinaToCosmosHas(ctx, act.FeePayer)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	return true, nil
}

func (k *Keeper) isValidWithdrawal(ctx context.Context, act *FetchedAction) (bool, error) {

	exists, err := k.keyRegistryKeeper.UserMinaToCosmosHas(ctx, act.FeePayer)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	cosmosPubKey, err := k.keyRegistryKeeper.UserGetMinaToCosmos(ctx, act.FeePayer)
	if err != nil {
		return false, err
	}

	addr, err := userAddressFromCosmosPubKey(cosmosPubKey)
	if err != nil {
		return false, err
	}

	if k.bankKeeper.SpendableCoins(ctx, addr).AmountOf(types.Denom).Uint64() < uint64(act.Amount) {
		return false, nil
	}

	return true, nil
}

func (k *Keeper) apply(ctx context.Context, act *FetchedAction) error {

	if k.bankKeeper == nil {
		return types.ErrBankKeeperNotConfigured
	}

	switch act.actionType {
	case DEPOSIT:
		return k.applyDeposit(ctx, act)
	case WITHDRAW:
		return k.applyWithdrawal(ctx, act)
	default:
		return types.ErrUnspecified
	}
}

func (k *Keeper) applyDeposit(ctx context.Context, act *FetchedAction) error {

	cosmosPubKey, err := k.keyRegistryKeeper.UserGetMinaToCosmos(ctx, act.FeePayer)
	if err != nil {
		return err
	}

	addr, err := userAddressFromCosmosPubKey(cosmosPubKey)
	if err != nil {
		return err
	}

	coins := sdk.NewCoins(sdk.NewCoin(types.Denom, math.NewInt(act.Amount)))

	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
		return err
	}

	return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, addr, coins)
}

func (k *Keeper) applyWithdrawal(ctx context.Context, act *FetchedAction) error {

	cosmosPubKey, err := k.keyRegistryKeeper.UserGetMinaToCosmos(ctx, act.FeePayer)
	if err != nil {
		return err
	}

	addr, err := userAddressFromCosmosPubKey(cosmosPubKey)
	if err != nil {
		return err
	}

	spendableCoins := k.bankKeeper.SpendableCoins(ctx, addr)

	minaAmount := spendableCoins.AmountOf(types.Denom)

	err = spendableCoins.Validate()
	if err != nil {
		return err
	}

	if minaAmount.Uint64() < uint64(act.Amount) {
		return types.ErrNotEnoughBalance
	}

	coins := sdk.NewCoins(sdk.NewCoin(types.Denom, math.NewInt(act.Amount)))

	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, addr, types.ModuleName, coins); err != nil {
		return err
	}

	return k.bankKeeper.BurnCoins(ctx, types.ModuleName, coins)
}

func userAddressFromCosmosPubKey(cosmosPubKey []byte) (sdk.AccAddress, error) {
	if len(cosmosPubKey) != secp256k1.PubKeySize {
		return nil, errors.Wrap(keyregistryTypes.ErrInvalidPublicKey, "user cosmos public key must be compressed secp256k1 (33 bytes)")
	}

	pubKey := secp256k1.PubKey{Key: cosmosPubKey}

	return sdk.AccAddress(pubKey.Address()), nil
}
