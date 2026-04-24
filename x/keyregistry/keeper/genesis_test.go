package keeper_test

import (
	"testing"

	cometed25519 "github.com/cometbft/cometbft/crypto/ed25519"
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

	userCosmosPubKey := secp256k1.GenPrivKey().PubKey()

	validatorPublicKey := cometed25519.GenPrivKey().PubKey()

	MinaPriv := secp256k1.GenPrivKey()
	minaPubKey := MinaPriv.PubKey().Bytes()

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		UserCosmosToMina: []*types.UserPublicKeyPair{
			{
				MinaKey:   minaPubKey,
				CosmosKey: userCosmosPubKey.Bytes(),
			},
		},
		UserMinaToCosmos: []*types.UserPublicKeyPair{
			{
				MinaKey:   minaPubKey,
				CosmosKey: userCosmosPubKey.Bytes(),
			},
		},
		ValidatorCosmosToMina: []*types.ValidatorPublicKeyPair{
			{
				MinaKey:   minaPubKey,
				CosmosKey: validatorPublicKey.Bytes(),
			},
		},
		ValidatorMinaToCosmos: []*types.ValidatorPublicKeyPair{
			{
				MinaKey:   minaPubKey,
				CosmosKey: validatorPublicKey.Bytes(),
			},
		},
	}

	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)
	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.EqualExportedValues(t, genesisState.UserCosmosToMina, got.UserCosmosToMina)
	require.EqualExportedValues(t, genesisState.UserMinaToCosmos, got.UserMinaToCosmos)
	require.EqualExportedValues(t, genesisState.ValidatorCosmosToMina, got.ValidatorCosmosToMina)
	require.EqualExportedValues(t, genesisState.ValidatorMinaToCosmos, got.ValidatorMinaToCosmos)

}
