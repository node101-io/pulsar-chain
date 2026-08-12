package keeper

import (
	"context"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/publickey"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
	keyregistryTypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func (k *Keeper) validateAction(ctx context.Context, act types.Action) ([]byte, bool, error) {
	if k.keyRegistryKeeper == nil {
		return nil, false, types.ErrKeyRegistryKeeperNotConfigured
	}

	if act.Amount <= 0 || act.BlockHeight <= 0 {
		return nil, false, nil
	}

	if act.ActionType != types.ActionType_ACTION_TYPE_DEPOSIT &&
		act.ActionType != types.ActionType_ACTION_TYPE_WITHDRAW {
		return nil, false, nil
	}

	minaPublicKey, err := coordinatesToPublicKey(act.XCoordinate, act.IsOdd)
	if err != nil {
		// Treat malformed public-key coordinates as an invalid action so the batch can continue.
		return nil, false, nil
	}

	var valid bool
	switch act.ActionType {
	case types.ActionType_ACTION_TYPE_DEPOSIT:
		valid, err = k.isValidDeposit(ctx, minaPublicKey)
	case types.ActionType_ACTION_TYPE_WITHDRAW:
		valid, err = k.isValidWithdrawal(ctx, act, minaPublicKey)
	}
	if err != nil {
		return nil, false, err
	}
	if !valid {
		return nil, false, nil
	}

	return minaPublicKey, true, nil
}

func coordinatesToPublicKey(xCoordinate []byte, isOdd bool) ([]byte, error) {
	fieldElement, err := field.NewFieldElement(xCoordinate)
	if err != nil {
		return nil, err
	}

	// For this case, networkID is unnecessary. Hence, we give empty string
	pubKey, err := publickey.NewPublicKeyFromFieldElement(fieldElement, isOdd, "")
	if err != nil {
		return nil, err
	}

	return pubKey.Bytes(), nil
}

func (k *Keeper) isValidDeposit(ctx context.Context, minaPublicKey []byte) (bool, error) {
	exists, err := k.keyRegistryKeeper.UserMinaToCosmosHas(ctx, minaPublicKey)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (k *Keeper) isValidWithdrawal(ctx context.Context, act types.Action, minaPublicKey []byte) (bool, error) {
	exists, err := k.keyRegistryKeeper.UserMinaToCosmosHas(ctx, minaPublicKey)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	cosmosPubKey, err := k.keyRegistryKeeper.UserGetMinaToCosmos(ctx, minaPublicKey)
	if err != nil {
		return false, err
	}

	addr, err := userAddressFromCosmosPubKey(cosmosPubKey)
	if err != nil {
		return false, err
	}

	// sdk.DefaultBondDenom's default value is "stake"
	// However, it gets overwritten once the app package starts (init func at app/config.go)
	// Also added a test for this app/config_external_test.go
	denom := sdk.DefaultBondDenom
	required := math.NewInt(act.Amount)
	if k.bankKeeper.SpendableCoins(ctx, addr).AmountOf(denom).LT(required) {
		return false, nil
	}

	return true, nil
}

func (k *Keeper) apply(ctx context.Context, act types.Action, minaPublicKey []byte) error {
	if k.bankKeeper == nil {
		return types.ErrBankKeeperNotConfigured
	}

	switch act.ActionType {
	case types.ActionType_ACTION_TYPE_DEPOSIT:
		return k.applyDeposit(ctx, act, minaPublicKey)
	case types.ActionType_ACTION_TYPE_WITHDRAW:
		return k.applyWithdrawal(ctx, act, minaPublicKey)
	default:
		return types.ErrUnspecified
	}
}

func (k *Keeper) applyDeposit(ctx context.Context, act types.Action, minaPublicKey []byte) error {
	cosmosPubKey, err := k.keyRegistryKeeper.UserGetMinaToCosmos(ctx, minaPublicKey)
	if err != nil {
		return err
	}

	addr, err := userAddressFromCosmosPubKey(cosmosPubKey)
	if err != nil {
		return err
	}

	denom := sdk.DefaultBondDenom
	coins := sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(act.Amount)))

	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
		return err
	}

	return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, addr, coins)
}

func (k *Keeper) applyWithdrawal(ctx context.Context, act types.Action, minaPublicKey []byte) error {
	cosmosPubKey, err := k.keyRegistryKeeper.UserGetMinaToCosmos(ctx, minaPublicKey)
	if err != nil {
		return err
	}

	addr, err := userAddressFromCosmosPubKey(cosmosPubKey)
	if err != nil {
		return err
	}

	spendableCoins := k.bankKeeper.SpendableCoins(ctx, addr)

	denom := sdk.DefaultBondDenom
	minaAmount := spendableCoins.AmountOf(denom)

	err = spendableCoins.Validate()
	if err != nil {
		return err
	}

	required := math.NewInt(act.Amount)
	if minaAmount.LT(required) {
		return types.ErrNotEnoughBalance
	}

	coins := sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(act.Amount)))

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
