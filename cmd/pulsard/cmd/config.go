package cmd

import (
	cmtcfg "github.com/cometbft/cometbft/config"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"

	"github.com/node101-io/pulsar-chain/x/verification/sidecar"
)

// initCometBFTConfig helps to override default CometBFT Config values.
// return cmtcfg.DefaultConfig if no custom configuration is required for the application.
func initCometBFTConfig() *cmtcfg.Config {
	cfg := cmtcfg.DefaultConfig()

	// Browsers talk to the node directly; without this they discard responses
	// the node answered fine.
	cfg.RPC.CORSAllowedOrigins = []string{"*"}

	// these values put a higher strain on node memory
	// cfg.P2P.MaxNumInboundPeers = 100
	// cfg.P2P.MaxNumOutboundPeers = 40

	return cfg
}

// initAppConfig helps to override default appConfig template and configs.
// return "", nil if no custom configuration is required for the application.
func initAppConfig() (string, interface{}) {
	// The following code snippet is just for reference.
	type CustomAppConfig struct {
		serverconfig.Config `mapstructure:",squash"`
		Bridge              bridgeConfig       `mapstructure:"bridge"`
		Verification        verificationConfig `mapstructure:"verification"`
	}

	// Optionally allow the chain developer to overwrite the SDK's default
	// server config.
	srvCfg := serverconfig.DefaultConfig()
	// The SDK's default minimum gas price is set to "" (empty value) inside
	// app.toml. If left empty by validators, the node will halt on startup.
	// However, the chain developer can set a default app.toml value for their
	// validators here.
	//
	// In summary:
	// - if you leave srvCfg.MinGasPrices = "", all validators MUST tweak their
	//   own app.toml config,
	// - if you set srvCfg.MinGasPrices non-empty, validators CAN tweak their
	//   own app.toml to override, or use this default value.
	//
	// In tests, we set the min gas prices to 0.
	// srvCfg.MinGasPrices = "0stake"

	// REST serves what a browser cannot reach over gRPC. The listen address
	// stays at its localhost default.
	srvCfg.API.Enable = true
	srvCfg.API.EnableUnsafeCORS = true

	customAppConfig := CustomAppConfig{
		Config: *srvCfg,
		Bridge: bridgeConfig{},
		Verification: verificationConfig{
			GRPCTransportMode: string(sidecar.TransportModeLoopback),
			RequestTimeout:    "100ms",
		},
	}

	customAppTemplate := serverconfig.DefaultConfigTemplate + bridgeConfigTemplate + verificationConfigTemplate
	// Edit the default template file
	//
	// customAppTemplate := serverconfig.DefaultConfigTemplate + `
	// [wasm]
	// # This is the maximum sdk gas (wasm and storage) that we allow for any x/wasm "smart" queries
	// query_gas_limit = 300000
	// # This is the number of wasm vm instances we keep cached in memory for speed-up
	// # Warning: this is currently unstable and may lead to crashes, best to keep for 0 unless testing locally
	// lru_size = 0`

	return customAppTemplate, customAppConfig
}

type bridgeConfig struct {
	WrapperGRPCAddress       string `mapstructure:"wrapper_grpc_address"`
	WrapperGRPCTransportMode string `mapstructure:"wrapper_grpc_transport_mode"`
}

type verificationConfig struct {
	Enabled           bool   `mapstructure:"enabled"`
	GRPCAddress       string `mapstructure:"grpc_address"`
	GRPCTransportMode string `mapstructure:"grpc_transport_mode"`
	RequestTimeout    string `mapstructure:"request_timeout"`
}

const bridgeConfigTemplate = `

###############################################################################
###                              Bridge Configuration                       ###
###############################################################################

[bridge]
wrapper_grpc_address = "{{ .Bridge.WrapperGRPCAddress }}"
wrapper_grpc_transport_mode = "{{ .Bridge.WrapperGRPCTransportMode }}"
`

const verificationConfigTemplate = `

###############################################################################
###                         Verification Configuration                      ###
###############################################################################

[verification]
enabled = {{ .Verification.Enabled }}
grpc_address = "{{ .Verification.GRPCAddress }}"
# trusted-network is plaintext and requires an operator-controlled private network.
grpc_transport_mode = "{{ .Verification.GRPCTransportMode }}"
request_timeout = "{{ .Verification.RequestTimeout }}"
`
