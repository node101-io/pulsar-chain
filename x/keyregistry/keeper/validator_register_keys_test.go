package keeper_test

import (
	"testing"

	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	keyregistrytypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

// TestValidatorRegisterKeysFail verifies that RegisterKeys fails with ErrInvalidPublicKey
// when the provided cosmos consensus public key is invalid.
func TestValidatorRegisterKeysFail(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateValidatorCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	creator, _, minaPubKey, cosmosSig, minaSig, err := signValidatorRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	_, err = ms.RegisterKeys(f.ctx, &keyregistrytypes.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: []byte("bad-cosmos-key"),
		MinaPublicKey:   minaPubKey,
		ActorType:       keyregistrytypes.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, keyregistrytypes.ErrInvalidPublicKey)
}

func TestValidatorRegisterKeysSuccess(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateValidatorCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	creator, cosmosPubKey, minaPubKey, cosmosSig, minaSig, err := signValidatorRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	resp, err := ms.RegisterKeys(f.ctx, &keyregistrytypes.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       keyregistrytypes.ActorType_VALIDATOR,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	exists, err := f.keeper.ValidatorCosmosToMinaHas(f.ctx, cosmosPubKey)
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = f.keeper.ValidatorMinaToCosmosHas(f.ctx, minaPubKey)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestValidatorInvalidCreatorAddress(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateValidatorCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	_, cosmosPubKey, minaPubKey, cosmosSig, minaSig, err := signValidatorRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	_, err = ms.RegisterKeys(f.ctx, &keyregistrytypes.MsgRegisterKeys{
		Creator:         "creator",
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       keyregistrytypes.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, keyregistrytypes.ErrInvalidCreatorAddress)
}

func TestValidatorInvalidSignature(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateValidatorCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	creator, cosmosPubKey, minaPubKey, _, minaSig, err := signValidatorRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	wrongCosmosSig, err := generateValidatorCosmosPrivKey().Sign(minaPubKey)
	require.NoError(t, err)

	_, err = ms.RegisterKeys(f.ctx, &keyregistrytypes.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: wrongCosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       keyregistrytypes.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, keyregistrytypes.ErrInvalidSignature)
}

func TestValidatorMalformedMinaSignature(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateValidatorCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	creator, cosmosPubKey, minaPubKey, cosmosSig, _, err := signValidatorRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	_, err = ms.RegisterKeys(f.ctx, &keyregistrytypes.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   malformedMinaSignature(),
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       keyregistrytypes.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, keyregistrytypes.ErrInvalidSignature)
}

func TestValidatorInsertSecondaryKeysFail(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateValidatorCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	creator, cosmosPubKey, minaPubKey, cosmosSig, minaSig, err := signValidatorRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	resp, err := ms.RegisterKeys(f.ctx, &keyregistrytypes.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       keyregistrytypes.ActorType_VALIDATOR,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	_, err = ms.RegisterKeys(f.ctx, &keyregistrytypes.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       keyregistrytypes.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, keyregistrytypes.ErrValidatorSecondaryKeyExists)
}
