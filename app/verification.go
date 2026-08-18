package app

import (
	"crypto/rand"
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
	verificationTimeoutOption  = "verification.sidecar-timeout"
	verificationStoreOption    = "verification.local-state-store"
	verificationStateFileName  = "verification_commitment_state.json"
)

func verificationProvider(appOpts servertypes.AppOptions) sidecar.Provider {
	if provider, ok := appOpts.Get(verificationProviderOption).(sidecar.Provider); ok && provider != nil {
		return provider
	}
	return sidecar.DisabledProvider{}
}

func verificationTimeout(appOpts servertypes.AppOptions) time.Duration {
	switch value := appOpts.Get(verificationTimeoutOption).(type) {
	case time.Duration:
		if value > 0 {
			return value
		}
	case string:
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return abci.DefaultVerificationSidecarTimeout
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
) (*verificationvalidator.Builder, error) {
	store, err := verificationStateStore(appOpts)
	if err != nil {
		return nil, err
	}
	return verificationvalidator.NewBuilder(
		keeper,
		verificationProvider(appOpts),
		store,
		rand.Reader,
		verificationTimeout(appOpts),
	)
}
