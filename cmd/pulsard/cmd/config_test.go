package cmd

import (
	"reflect"
	"testing"

	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	"github.com/stretchr/testify/require"
)

// curl cannot see a missing CORS header, so these are easy to switch off by
// accident — API.Enable had already been pinned to false elsewhere.
func TestInitCometBFTConfigAllowsBrowserOrigins(t *testing.T) {
	require.Equal(t, []string{"*"}, initCometBFTConfig().RPC.CORSAllowedOrigins)
}

func TestInitAppConfigServesAPIWithCORS(t *testing.T) {
	_, appConfig := initAppConfig()

	// CustomAppConfig is declared inside initAppConfig, so reach the embedded
	// config by name.
	embedded := reflect.ValueOf(appConfig).FieldByName("Config")
	require.True(t, embedded.IsValid(), "app config should embed serverconfig.Config")

	srvCfg, ok := embedded.Interface().(serverconfig.Config)
	require.True(t, ok)
	require.True(t, srvCfg.API.Enable)
	require.True(t, srvCfg.API.EnableUnsafeCORS)
}
