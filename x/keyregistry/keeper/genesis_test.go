package keeper_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"

	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
	}

	f := initFixture(t)
	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)
	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.EqualExportedValues(t, genesisState.Params, got.Params)
}

// TestInitAndExportGenesis verifies that a genesis state can be initialized
// and later exported without losing the registered key pairs.
func TestInitAndExportGenesis(t *testing.T) {

	f := initFixture(t)

	cosmosPriv := secp256k1.GenPrivKey()

	cosmosPubKey := cosmosPriv.PubKey()

	minaPubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		KeyPairs: []*types.KeyPair{
			{
				MinaKey:   minaPubKey,
				CosmosKey: cosmosPubKey.Bytes(),
			},
		},
	}

	err = f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)
	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.EqualExportedValues(t, genesisState.KeyPairs, got.KeyPairs)

}
