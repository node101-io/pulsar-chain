package ante_test

import (
	"context"
	"testing"
	"time"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	txsigning "cosmossdk.io/x/tx/signing"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	appante "github.com/node101-io/pulsar-chain/app/ante"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
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

var stubKeyregistryKeeper = &keyregistrykeeper.Keeper{}

// A fully populated HandlerOptions should construct a usable ante chain.
// This is the baseline success case that all of the stricter validation tests compare against.
func TestNewAnteHandler(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		AccountKeeper:     stubAccountKeeper{},
		BankKeeper:        stubBankKeeper{},
		SignModeHandler:   &txsigning.HandlerMap{},
		KeyregistryKeeper: stubKeyregistryKeeper,
		MinaNetworkID:     appante.DefaultMinaNetworkID,
		Logger:            log.NewNopLogger(),
	})

	require.NoError(t, err)
	require.NotNil(t, anteHandler)
}

// AccountKeeper is required because the auth ante stack depends on account state almost everywhere.
// Failing at construction time is safer than letting a partially wired handler reach runtime.
func TestNewAnteHandlerRequiresAccountKeeper(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		BankKeeper:        stubBankKeeper{},
		SignModeHandler:   &txsigning.HandlerMap{},
		KeyregistryKeeper: stubKeyregistryKeeper,
		MinaNetworkID:     appante.DefaultMinaNetworkID,
		Logger:            log.NewNopLogger(),
	})

	require.ErrorContains(t, err, "account keeper is required for ante builder")
	require.Nil(t, anteHandler)
}

// BankKeeper is mandatory for fee deduction in the default auth decorators.
// This test ensures the constructor rejects missing fee-transfer dependencies immediately.
func TestNewAnteHandlerRequiresBankKeeper(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		AccountKeeper:     stubAccountKeeper{},
		SignModeHandler:   &txsigning.HandlerMap{},
		KeyregistryKeeper: stubKeyregistryKeeper,
		MinaNetworkID:     appante.DefaultMinaNetworkID,
		Logger:            log.NewNopLogger(),
	})

	require.ErrorContains(t, err, "bank keeper is required for ante builder")
	require.Nil(t, anteHandler)
}

// SignModeHandler is needed by both the Cosmos and Mina signature verification paths.
// Missing it would make sign-byte generation impossible later in the ante chain.
func TestNewAnteHandlerRequiresSignModeHandler(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		AccountKeeper:     stubAccountKeeper{},
		BankKeeper:        stubBankKeeper{},
		KeyregistryKeeper: stubKeyregistryKeeper,
		MinaNetworkID:     appante.DefaultMinaNetworkID,
		Logger:            log.NewNopLogger(),
	})

	require.ErrorContains(t, err, "sign mode handler is required for ante builder")
	require.Nil(t, anteHandler)
}

// KeyregistryKeeper is part of the custom verifier contract for Mina-authenticated txs.
// The constructor should refuse to build an ante handler that can never resolve Mina signers.
func TestNewAnteHandlerRequiresKeyregistryKeeper(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		AccountKeeper:   stubAccountKeeper{},
		BankKeeper:      stubBankKeeper{},
		SignModeHandler: &txsigning.HandlerMap{},
		MinaNetworkID:   appante.DefaultMinaNetworkID,
		Logger:          log.NewNopLogger(),
	})

	require.ErrorContains(t, err, "keyregistry keeper is required for ante builder")
	require.Nil(t, anteHandler)
}

// Mina network selection affects how Mina signatures are verified on-chain.
// This test guards against silently constructing a verifier with an empty network ID.
func TestNewAnteHandlerRequiresMinaNetworkID(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		AccountKeeper:     stubAccountKeeper{},
		BankKeeper:        stubBankKeeper{},
		SignModeHandler:   &txsigning.HandlerMap{},
		KeyregistryKeeper: stubKeyregistryKeeper,
		Logger:            log.NewNopLogger(),
	})

	require.ErrorContains(t, err, "mina network ID is required for ante builder")
	require.Nil(t, anteHandler)
}

// Mina network IDs reach this constructor from runtime configuration. Rejecting
// unknown values at startup prevents an unusable wallet verifier from being installed.
func TestNewAnteHandlerRejectsUnsupportedMinaNetworkID(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		AccountKeeper:     stubAccountKeeper{},
		BankKeeper:        stubBankKeeper{},
		SignModeHandler:   &txsigning.HandlerMap{},
		KeyregistryKeeper: stubKeyregistryKeeper,
		MinaNetworkID:     "unsupported",
		Logger:            log.NewNopLogger(),
	})

	require.ErrorIs(t, err, sdkerrors.ErrLogic)
	require.ErrorContains(t, err, "unsupported Mina network ID")
	require.Nil(t, anteHandler)
}

// The custom verifier emits debug information when verification is skipped.
// Requiring a logger up front avoids nil logger surprises during ante execution.
func TestNewAnteHandlerRequiresLogger(t *testing.T) {
	t.Parallel()

	anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
		AccountKeeper:     stubAccountKeeper{},
		BankKeeper:        stubBankKeeper{},
		SignModeHandler:   &txsigning.HandlerMap{},
		KeyregistryKeeper: stubKeyregistryKeeper,
		MinaNetworkID:     appante.DefaultMinaNetworkID,
	})

	require.ErrorContains(t, err, "logger is required for ante builder")
	require.Nil(t, anteHandler)
}
