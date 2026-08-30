package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/abci"
	verificationkeeper "github.com/node101-io/pulsar-chain/x/verification/keeper"
	"github.com/node101-io/pulsar-chain/x/verification/sidecar"
	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
	verificationvalidator "github.com/node101-io/pulsar-chain/x/verification/validator"
)

type verificationAppOptions map[string]any

func (options verificationAppOptions) Get(key string) any {
	return options[key]
}

func TestVerificationProviderOption(t *testing.T) {
	called := false
	provider := sidecar.ProviderFunc(func(context.Context, [][]byte) ([]sidecar.VerificationResult, error) {
		called = true
		return nil, nil
	})

	configured, client, active, err := verificationProvider(verificationAppOptions{verificationProviderOption: provider})
	require.NoError(t, err)
	require.Nil(t, client)
	require.True(t, active)
	_, err = configured.GetVerificationResults(context.Background(), nil)
	require.NoError(t, err)
	require.True(t, called)

	disabled, client, active, err := verificationProvider(verificationAppOptions{})
	require.NoError(t, err)
	require.Nil(t, client)
	require.False(t, active)
	_, ok := disabled.(sidecar.DisabledProvider)
	require.True(t, ok)
}

func TestVerificationTimeoutOption(t *testing.T) {
	for _, value := range []any{250 * time.Millisecond, "300ms"} {
		timeout, err := verificationTimeout(verificationAppOptions{sidecar.RequestTimeoutConfigKey: value}, true)
		require.NoError(t, err)
		require.Positive(t, timeout)
	}
	for _, value := range []any{time.Duration(0), "invalid", "0s", "2s"} {
		_, err := verificationTimeout(verificationAppOptions{sidecar.RequestTimeoutConfigKey: value}, true)
		require.Error(t, err)
		timeout, err := verificationTimeout(verificationAppOptions{sidecar.RequestTimeoutConfigKey: value}, false)
		require.NoError(t, err)
		require.Equal(t, abci.DefaultVerificationSidecarTimeout, timeout)
	}
}

func TestVerificationProviderEnabledConfiguration(t *testing.T) {
	provider, client, active, err := verificationProvider(verificationAppOptions{
		sidecar.EnabledConfigKey:       true,
		sidecar.GRPCAddressConfigKey:   "127.0.0.1:1",
		sidecar.TransportModeConfigKey: "loopback",
	})
	require.NoError(t, err)
	require.True(t, active)
	require.Same(t, client, provider)
	require.NoError(t, client.Close())

	_, _, _, err = verificationProvider(verificationAppOptions{
		sidecar.EnabledConfigKey:       true,
		sidecar.TransportModeConfigKey: "loopback",
	})
	require.ErrorIs(t, err, sidecar.ErrInvalidGRPCAddress)
}

func TestVerificationStateStoreUsesNodeHomeAndSupportsInjection(t *testing.T) {
	home := t.TempDir()
	store, err := verificationStateStore(verificationAppOptions{flags.FlagHome: home})
	require.NoError(t, err)
	fileStore, ok := store.(*verificationvalidator.FileStore)
	require.True(t, ok)
	require.Equal(t, filepath.Join(home, "data", verificationStateFileName), fileStore.Path())

	injected := verificationvalidator.NewMemoryStore()
	store, err = verificationStateStore(verificationAppOptions{verificationStoreOption: injected})
	require.NoError(t, err)
	require.Same(t, injected, store)
}

func TestDisabledVerificationUsesConcreteBuilderWithoutJournal(t *testing.T) {
	home := t.TempDir()
	builder, err := newVerificationBuilder(
		verificationAppOptions{flags.FlagHome: home},
		verificationkeeper.Keeper{},
		sidecar.DisabledProvider{},
		false,
		abci.DefaultVerificationSidecarTimeout,
	)
	require.NoError(t, err)
	require.IsType(t, verificationvalidator.DisabledBuilder{}, builder)
	_, err = os.Stat(filepath.Join(home, "data", verificationStateFileName))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestEnabledVerificationRejectsUnreadableJournal(t *testing.T) {
	store := verificationvalidator.NewMemoryStore()
	expectedErr := errors.New("journal unavailable")
	store.SetLoadError(expectedErr)

	_, err := newVerificationBuilder(
		verificationAppOptions{verificationStoreOption: store},
		verificationkeeper.Keeper{},
		sidecar.DisabledProvider{},
		true,
		abci.DefaultVerificationSidecarTimeout,
	)
	require.ErrorIs(t, err, expectedErr)
}

func TestVerificationModuleIsWiredIntoApplication(t *testing.T) {
	wrapperAddress := startArchiveWrapperHealthServer(t, 0)
	pulsarApp := newZeroHeightExportTestApp(t, wrapperAddress)

	require.NotNil(t, pulsarApp.GetKey(verificationtypes.StoreKey))
	_, registered := pulsarApp.ModuleManager.Modules[verificationtypes.ModuleName]
	require.True(t, registered)
	_, hasGenesis := pulsarApp.DefaultGenesis()[verificationtypes.ModuleName]
	require.True(t, hasGenesis)
}

func TestEnabledVerificationClientIsWiredLazily(t *testing.T) {
	wrapperAddress := startArchiveWrapperHealthServer(t, 0)
	pulsarApp := newZeroHeightExportTestAppWithOptions(t, wrapperAddress, map[string]any{
		sidecar.EnabledConfigKey:        true,
		sidecar.GRPCAddressConfigKey:    "127.0.0.1:1",
		sidecar.TransportModeConfigKey:  "loopback",
		sidecar.RequestTimeoutConfigKey: "100ms",
	})
	require.NotNil(t, pulsarApp.VerificationSidecarClient)
}

func TestEnabledVerificationRejectsInvalidConfig(t *testing.T) {
	wrapperAddress := startArchiveWrapperHealthServer(t, 0)
	require.Panics(t, func() {
		newZeroHeightExportTestAppWithOptions(t, wrapperAddress, map[string]any{
			sidecar.EnabledConfigKey:       true,
			sidecar.TransportModeConfigKey: "loopback",
		})
	})
}
