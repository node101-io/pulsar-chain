package ante_test

import (
	"context"
	"testing"
	"time"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	txsigning "cosmossdk.io/x/tx/signing"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	appante "github.com/node101-io/pulsar-chain/app/ante"
)

type stubAccountKeeper struct{}

func (stubAccountKeeper) GetParams(context.Context) authtypes.Params {
	return authtypes.Params{}
}

func (stubAccountKeeper) GetAccount(context.Context, sdk.AccAddress) sdk.AccountI {
	return nil
}

func (stubAccountKeeper) SetAccount(context.Context, sdk.AccountI) {}

func (stubAccountKeeper) GetModuleAddress(string) sdk.AccAddress {
	return nil
}

func (stubAccountKeeper) AddressCodec() address.Codec {
	return nil
}

func (stubAccountKeeper) UnorderedTransactionsEnabled() bool {
	return false
}

func (stubAccountKeeper) RemoveExpiredUnorderedNonces(sdk.Context) error {
	return nil
}

func (stubAccountKeeper) TryAddUnorderedNonce(sdk.Context, []byte, time.Time) error {
	return nil
}

type stubBankKeeper struct{}

func (stubBankKeeper) IsSendEnabledCoins(context.Context, ...sdk.Coin) error {
	return nil
}

func (stubBankKeeper) SendCoins(context.Context, sdk.AccAddress, sdk.AccAddress, sdk.Coins) error {
	return nil
}

func (stubBankKeeper) SendCoinsFromAccountToModule(context.Context, sdk.AccAddress, string, sdk.Coins) error {
	return nil
}

var (
	_ authante.AccountKeeper = stubAccountKeeper{}
	_ authtypes.BankKeeper   = stubBankKeeper{}
)

type stubMinaAddressResolver struct{}

func (stubMinaAddressResolver) GetCosmosToMina(context.Context, []byte) ([]byte, error) {
	return nil, nil
}

func TestNewAnteHandler(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		AccountKeeper:       stubAccountKeeper{},
		BankKeeper:          stubBankKeeper{},
		SignModeHandler:     &txsigning.HandlerMap{},
		MinaAddressResolver: stubMinaAddressResolver{},
		MinaNetworkID:       appante.DefaultMinaNetworkID,
		Logger:              log.NewNopLogger(),
	})

	require.NoError(t, err)
	require.NotNil(t, anteHandler)
}

func TestNewAnteHandlerRequiresAccountKeeper(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		BankKeeper:          stubBankKeeper{},
		SignModeHandler:     &txsigning.HandlerMap{},
		MinaAddressResolver: stubMinaAddressResolver{},
		MinaNetworkID:       appante.DefaultMinaNetworkID,
		Logger:              log.NewNopLogger(),
	})

	require.ErrorContains(t, err, "account keeper is required for ante builder")
	require.Nil(t, anteHandler)
}

func TestNewAnteHandlerRequiresBankKeeper(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		AccountKeeper:       stubAccountKeeper{},
		SignModeHandler:     &txsigning.HandlerMap{},
		MinaAddressResolver: stubMinaAddressResolver{},
		MinaNetworkID:       appante.DefaultMinaNetworkID,
		Logger:              log.NewNopLogger(),
	})

	require.ErrorContains(t, err, "bank keeper is required for ante builder")
	require.Nil(t, anteHandler)
}

func TestNewAnteHandlerRequiresSignModeHandler(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		AccountKeeper:       stubAccountKeeper{},
		BankKeeper:          stubBankKeeper{},
		MinaAddressResolver: stubMinaAddressResolver{},
		MinaNetworkID:       appante.DefaultMinaNetworkID,
		Logger:              log.NewNopLogger(),
	})

	require.ErrorContains(t, err, "sign mode handler is required for ante builder")
	require.Nil(t, anteHandler)
}

func TestNewAnteHandlerRequiresMinaAddressResolver(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		AccountKeeper:   stubAccountKeeper{},
		BankKeeper:      stubBankKeeper{},
		SignModeHandler: &txsigning.HandlerMap{},
		MinaNetworkID:   appante.DefaultMinaNetworkID,
		Logger:          log.NewNopLogger(),
	})

	require.ErrorContains(t, err, "mina address resolver is required for ante builder")
	require.Nil(t, anteHandler)
}

func TestNewAnteHandlerRequiresMinaNetworkID(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		AccountKeeper:       stubAccountKeeper{},
		BankKeeper:          stubBankKeeper{},
		SignModeHandler:     &txsigning.HandlerMap{},
		MinaAddressResolver: stubMinaAddressResolver{},
		Logger:              log.NewNopLogger(),
	})

	require.ErrorContains(t, err, "mina network ID is required for ante builder")
	require.Nil(t, anteHandler)
}

func TestNewAnteHandlerRequiresLogger(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		AccountKeeper:       stubAccountKeeper{},
		BankKeeper:          stubBankKeeper{},
		SignModeHandler:     &txsigning.HandlerMap{},
		MinaAddressResolver: stubMinaAddressResolver{},
		MinaNetworkID:       appante.DefaultMinaNetworkID,
	})

	require.ErrorContains(t, err, "logger is required for ante builder")
	require.Nil(t, anteHandler)
}
