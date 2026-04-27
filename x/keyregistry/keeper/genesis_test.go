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

// TestInitAndExportGenesis verifies that a canonical genesis state can be
// initialized, stored in both runtime lookup maps, and exported again.
func TestInitAndExportGenesis(t *testing.T) {
	f := initFixture(t)

	userCosmosPubKey := secp256k1.GenPrivKey().PubKey()
	validatorPublicKey := cometed25519.GenPrivKey().PubKey()
	minaPriv := secp256k1.GenPrivKey()
	minaPubKey := minaPriv.PubKey().Bytes()

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		UserKeyPairs: []*types.UserPublicKeyPair{
			{
				MinaKey:   minaPubKey,
				CosmosKey: userCosmosPubKey.Bytes(),
			},
		},
		ValidatorKeyPairs: []*types.ValidatorPublicKeyPair{
			{
				MinaKey:   minaPubKey,
				CosmosKey: validatorPublicKey.Bytes(),
			},
		},
	}

	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)

	userMinaPubKey, err := f.keeper.UserGetCosmosToMina(f.ctx, userCosmosPubKey.Bytes())
	require.NoError(t, err)
	require.Equal(t, minaPubKey, userMinaPubKey)

	userCosmosKey, err := f.keeper.UserGetMinaToCosmos(f.ctx, minaPubKey)
	require.NoError(t, err)
	require.Equal(t, userCosmosPubKey.Bytes(), userCosmosKey)

	validatorMinaPubKey, err := f.keeper.ValidatorGetCosmosToMina(f.ctx, validatorPublicKey.Bytes())
	require.NoError(t, err)
	require.Equal(t, minaPubKey, validatorMinaPubKey)

	validatorCosmosKey, err := f.keeper.ValidatorGetMinaToCosmos(f.ctx, minaPubKey)
	require.NoError(t, err)
	require.Equal(t, validatorPublicKey.Bytes(), validatorCosmosKey)

	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.EqualExportedValues(t, genesisState.UserKeyPairs, got.UserKeyPairs)
	require.EqualExportedValues(t, genesisState.ValidatorKeyPairs, got.ValidatorKeyPairs)
}

func TestInitGenesisRejectsInvalidStateWithoutPartialWrite(t *testing.T) {
	f := initFixture(t)

	userCosmosPubKey := secp256k1.GenPrivKey().PubKey()
	minaPrivKey := secp256k1.GenPrivKey()
	minaPubKey := minaPrivKey.PubKey().Bytes()
	secondaryMinaPrivKey := secp256k1.GenPrivKey()
	secondaryMinaPubKey := secondaryMinaPrivKey.PubKey().Bytes()

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		UserKeyPairs: []*types.UserPublicKeyPair{
			{
				MinaKey:   minaPubKey,
				CosmosKey: userCosmosPubKey.Bytes(),
			},
			{
				MinaKey:   secondaryMinaPubKey,
				CosmosKey: userCosmosPubKey.Bytes(),
			},
		},
	}

	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.ErrorIs(t, err, types.ErrInvalidGenesisState)

	exists, err := f.keeper.UserCosmosToMinaHas(f.ctx, userCosmosPubKey.Bytes())
	require.NoError(t, err)
	require.False(t, exists)

	exists, err = f.keeper.UserMinaToCosmosHas(f.ctx, minaPubKey)
	require.NoError(t, err)
	require.False(t, exists)

	exists, err = f.keeper.UserMinaToCosmosHas(f.ctx, secondaryMinaPubKey)
	require.NoError(t, err)
	require.False(t, exists)
}
