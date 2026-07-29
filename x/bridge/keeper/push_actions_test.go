package keeper_test

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type mockBankKeeper struct {
	spendable sdk.Coins

	spendableCalls           int
	sendCoinsFromModuleCalls int
	sendCoinsToModuleCalls   int
	mintCoinsCalls           int
	burnCoinsCalls           int
}

func NewMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{
		spendable: sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 123)),
	}
}

func (b *mockBankKeeper) SpendableCoins(context.Context, sdk.AccAddress) sdk.Coins {
	b.spendableCalls++
	return b.spendable
}

func (b *mockBankKeeper) SendCoinsFromModuleToAccount(context.Context, string, sdk.AccAddress, sdk.Coins) error {
	b.sendCoinsFromModuleCalls++
	return nil
}

func (b *mockBankKeeper) SendCoinsFromAccountToModule(context.Context, sdk.AccAddress, string, sdk.Coins) error {
	b.sendCoinsToModuleCalls++
	return nil
}

func (b *mockBankKeeper) MintCoins(context.Context, string, sdk.Coins) error {
	b.mintCoinsCalls++
	return nil
}

func (b *mockBankKeeper) BurnCoins(context.Context, string, sdk.Coins) error {
	b.burnCoinsCalls++
	return nil
}
