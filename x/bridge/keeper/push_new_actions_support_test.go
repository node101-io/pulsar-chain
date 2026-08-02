package keeper_test

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type mockBankKeeper struct {
	spendable sdk.Coins

	spendableCalls    int
	lastSpendableAddr sdk.AccAddress

	sendCoinsFromModuleCalls int
	lastSendFromModule       string
	lastSendToAddr           sdk.AccAddress
	lastSendFromModuleCoins  sdk.Coins
	sendFromModuleErr        error

	sendCoinsToModuleCalls int
	lastSendFromAddr       sdk.AccAddress
	lastSendToModule       string
	lastSendToModuleCoins  sdk.Coins
	sendToModuleErr        error

	mintCoinsCalls int
	lastMintModule string
	lastMintCoins  sdk.Coins
	mintErr        error

	burnCoinsCalls int
	lastBurnModule string
	lastBurnCoins  sdk.Coins
	burnErr        error
}

func NewMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{
		spendable: sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 123)),
	}
}

func cloneCoins(coins sdk.Coins) sdk.Coins {
	if coins == nil {
		return nil
	}
	out := make(sdk.Coins, len(coins))
	copy(out, coins)
	return out
}

func cloneAddr(addr sdk.AccAddress) sdk.AccAddress {
	if addr == nil {
		return nil
	}
	out := make(sdk.AccAddress, len(addr))
	copy(out, addr)
	return out
}

func (b *mockBankKeeper) SpendableCoins(_ context.Context, addr sdk.AccAddress) sdk.Coins {
	b.spendableCalls++
	b.lastSpendableAddr = cloneAddr(addr)
	return cloneCoins(b.spendable)
}

func (b *mockBankKeeper) SendCoinsFromModuleToAccount(
	_ context.Context,
	module string,
	addr sdk.AccAddress,
	coins sdk.Coins,
) error {
	b.sendCoinsFromModuleCalls++
	b.lastSendFromModule = module
	b.lastSendToAddr = cloneAddr(addr)
	b.lastSendFromModuleCoins = cloneCoins(coins)
	return b.sendFromModuleErr
}

func (b *mockBankKeeper) SendCoinsFromAccountToModule(
	_ context.Context,
	addr sdk.AccAddress,
	module string,
	coins sdk.Coins,
) error {
	b.sendCoinsToModuleCalls++
	b.lastSendFromAddr = cloneAddr(addr)
	b.lastSendToModule = module
	b.lastSendToModuleCoins = cloneCoins(coins)
	return b.sendToModuleErr
}

func (b *mockBankKeeper) MintCoins(_ context.Context, module string, coins sdk.Coins) error {
	b.mintCoinsCalls++
	b.lastMintModule = module
	b.lastMintCoins = cloneCoins(coins)
	return b.mintErr
}

func (b *mockBankKeeper) BurnCoins(_ context.Context, module string, coins sdk.Coins) error {
	b.burnCoinsCalls++
	b.lastBurnModule = module
	b.lastBurnCoins = cloneCoins(coins)
	return b.burnErr
}
