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
	if provider, ok := appOpts.Get(verificationProviderOption).(sidecar.Provider); ok && provider != nil {
		return provider, nil, true, nil
	}
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
	if !active {
		return abci.DefaultVerificationSidecarTimeout, nil
	}
	return 0, fmt.Errorf("%s must be greater than zero and at most %s", sidecar.RequestTimeoutConfigKey, maxVerificationTimeout)
}

func verificationStateStore(appOpts servertypes.AppOptions) (verificationvalidator.StateStore, error) {
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
