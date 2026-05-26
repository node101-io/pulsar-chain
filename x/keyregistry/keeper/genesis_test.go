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

	userMinaPriv, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)

	userMinaPk, err := userMinaPriv.ToPublicKey()
	require.NoError(t, err)
	userMinaPubKey := userMinaPk.Bytes()

	validatorMinaPriv, err := generateMinaSecondaryKeyPair(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	validatorMinaPk, err := validatorMinaPriv.ToPublicKey()
	require.NoError(t, err)
	validatorMinaPubKey := validatorMinaPk.Bytes()

	genesisState := types.GenesisState{
		Params: types.DefaultParams(),
		UserKeyPairs: []*types.UserPublicKeyPair{
			{
				MinaKey:   userMinaPubKey,
				CosmosKey: userCosmosPubKey.Bytes(),
			},
		},
		ValidatorKeyPairs: []*types.ValidatorPublicKeyPair{
			{
				MinaKey:   validatorMinaPubKey,
				CosmosKey: validatorPublicKey.Bytes(),
			},
		},
	}

	err = f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)

	userMinaPubKeyGot, err := f.keeper.UserGetCosmosToMina(f.ctx, userCosmosPubKey.Bytes())
	require.NoError(t, err)
	require.Equal(t, userMinaPubKey, userMinaPubKeyGot)

	userCosmosKey, err := f.keeper.UserGetMinaToCosmos(f.ctx, userMinaPubKey)
	require.NoError(t, err)
	require.Equal(t, userCosmosPubKey.Bytes(), userCosmosKey)

	validatorMinaPubKeyGot, err := f.keeper.ValidatorGetCosmosToMina(f.ctx, validatorPublicKey.Bytes())
	require.NoError(t, err)
	require.Equal(t, validatorMinaPubKey, validatorMinaPubKeyGot)

	validatorCosmosKey, err := f.keeper.ValidatorGetMinaToCosmos(f.ctx, validatorMinaPubKey)
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

	minaPrivKey, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)

	minaPk, err := minaPrivKey.ToPublicKey()
	require.NoError(t, err)
	minaPubKey := minaPk.Bytes()

	secondaryMinaPrivKey, err := generateMinaSecondaryKeyPair(types.ActorType_USER)
	require.NoError(t, err)

	secondaryMinaPk, err := secondaryMinaPrivKey.ToPublicKey()
	require.NoError(t, err)
	secondaryMinaPubKey := secondaryMinaPk.Bytes()

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

	err = f.keeper.InitGenesis(f.ctx, genesisState)
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
