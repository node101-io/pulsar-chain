package app

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"

	"github.com/node101-io/pulsar-chain/abci"
	verificationkeeper "github.com/node101-io/pulsar-chain/x/verification/keeper"
	"github.com/node101-io/pulsar-chain/x/verification/sidecar"
	verificationvalidator "github.com/node101-io/pulsar-chain/x/verification/validator"
)

const (
	verificationProviderOption = "verification.sidecar-provider"
	verificationStoreOption    = "verification.local-state-store"
	verificationStateFileName  = "verification_commitment_state.json"
	maxVerificationTimeout     = time.Second
)

func verificationProvider(
	appOpts servertypes.AppOptions,
) (sidecar.Provider, *sidecar.Client, bool, error) {
	// Typed injection is primarily for tests and embedding. It takes precedence
	// over app.toml so an embedding application can provide an in-process or mock
	// verifier without opening a network connection. The app does not own or close
	// a provider created by its caller.
	if provider, ok := appOpts.Get(verificationProviderOption).(sidecar.Provider); ok && provider != nil {
		return provider, nil, true, nil
	}
	// Disabled is the safe default and does not require an endpoint. The returned
	// provider behaves like a sidecar with no completed results, which preserves
	// block production while producing no new verification commitments.
	enabled, _ := appOpts.Get(sidecar.EnabledConfigKey).(bool)
	if !enabled {
		return sidecar.DisabledProvider{}, nil, false, nil
	}
	address, _ := appOpts.Get(sidecar.GRPCAddressConfigKey).(string)
	modeValue, _ := appOpts.Get(sidecar.TransportModeConfigKey).(string)
	mode, err := sidecar.ParseTransportMode(modeValue)
	if err != nil {
		return nil, nil, true, err
	}
	// NewClient validates the endpoint but connects lazily. Operators therefore
	// receive an immediate error for unsafe configuration, while a correctly
	// configured sidecar that is temporarily offline does not prevent node startup.
	client, err := sidecar.NewClient(address, mode)
	if err != nil {
		return nil, nil, true, err
	}
	return client, client, true, nil
}

func verificationTimeout(appOpts servertypes.AppOptions, active bool) (time.Duration, error) {
	value := appOpts.Get(sidecar.RequestTimeoutConfigKey)
	if value == nil {
		return abci.DefaultVerificationSidecarTimeout, nil
	}
	var timeout time.Duration
	switch typed := value.(type) {
	case time.Duration:
		timeout = typed
	case string:
		parsed, err := time.ParseDuration(typed)
		if err == nil {
			timeout = parsed
		}
	}
	if timeout > 0 && timeout <= maxVerificationTimeout {
		return timeout, nil
	}
	// Ignore malformed dormant config because it cannot affect a disabled runtime,
	// but fail fast when verification is active. The one-second ceiling prevents
	// an operator setting from blocking ExtendVote long enough to harm consensus.
	if !active {
		return abci.DefaultVerificationSidecarTimeout, nil
	}
	return 0, fmt.Errorf("%s must be greater than zero and at most %s", sidecar.RequestTimeoutConfigKey, maxVerificationTimeout)
}

func verificationStateStore(appOpts servertypes.AppOptions) (verificationvalidator.StateStore, error) {
	// Like provider injection, an injected store is useful for tests and remains
	// owned by the caller. Production nodes use the bounded file-backed journal.
	if store, ok := appOpts.Get(verificationStoreOption).(verificationvalidator.StateStore); ok && store != nil {
		return store, nil
	}
	home, _ := appOpts.Get(flags.FlagHome).(string)
	if home == "" {
		home = DefaultNodeHome
	}
	absoluteHome, err := filepath.Abs(home)
	if err != nil {
		return nil, err
	}
	// Commitment preimages contain validator-private salts and therefore live in
	// the node data directory, separate from replicated application state. Every
	// validator may have different local entries without changing the app hash.
	return verificationvalidator.NewFileStore(filepath.Join(absoluteHome, "data", verificationStateFileName))
}

func newVerificationBuilder(
	appOpts servertypes.AppOptions,
	keeper verificationkeeper.Keeper,
	provider sidecar.Provider,
	timeout time.Duration,
) (*verificationvalidator.Builder, error) {
	store, err := verificationStateStore(appOpts)
	if err != nil {
		return nil, err
	}
	return verificationvalidator.NewBuilder(
		keeper,
		provider,
		store,
		rand.Reader,
		timeout,
	)
}
