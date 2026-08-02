package app_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	_ "github.com/node101-io/pulsar-chain/app"
	"github.com/stretchr/testify/require"
)

func TestImportingAppOverridesDefaultBondDenomToPMina(t *testing.T) {
	require.Equal(t, "pmina", sdk.DefaultBondDenom)
}
