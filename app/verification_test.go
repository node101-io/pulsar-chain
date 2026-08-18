package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/abci"
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

	configured := verificationProvider(verificationAppOptions{verificationProviderOption: provider})
	_, err := configured.GetVerificationResults(context.Background(), nil)
	require.NoError(t, err)
	require.True(t, called)

	disabled := verificationProvider(verificationAppOptions{})
	_, ok := disabled.(sidecar.DisabledProvider)
	require.True(t, ok)
}

func TestVerificationTimeoutOption(t *testing.T) {
	require.Equal(t, 250*time.Millisecond, verificationTimeout(verificationAppOptions{
		verificationTimeoutOption: 250 * time.Millisecond,
	}))
	require.Equal(t, 300*time.Millisecond, verificationTimeout(verificationAppOptions{
		verificationTimeoutOption: "300ms",
	}))
	for _, value := range []any{nil, time.Duration(0), "invalid", "0s"} {
		require.Equal(t, abci.DefaultVerificationSidecarTimeout, verificationTimeout(verificationAppOptions{
			verificationTimeoutOption: value,
		}))
	}
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

func TestVerificationModuleIsWiredIntoApplication(t *testing.T) {
	wrapperAddress := startArchiveWrapperHealthServer(t, 0)
	pulsarApp := newZeroHeightExportTestApp(t, wrapperAddress)

	require.NotNil(t, pulsarApp.GetKey(verificationtypes.StoreKey))
	_, registered := pulsarApp.ModuleManager.Modules[verificationtypes.ModuleName]
	require.True(t, registered)
	_, hasGenesis := pulsarApp.DefaultGenesis()[verificationtypes.ModuleName]
	require.True(t, hasGenesis)
}
