package keeper_test

import (
	"testing"

	cometed25519 "github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/node101-io/mina-signer-go/privatekey"

	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

func registerValidatorKeysForUpdateTest(t *testing.T, f *fixture, ms types.MsgServer) (cometed25519.PrivKey, []byte, *privatekey.PrivateKey) {
	t.Helper()

	cosmosPriv := generateValidatorCosmosPrivKey()

	minaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	newMinaPriv, err := generateMinaSecondaryKeyPair(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	creator, cosmosPubKey, minaPubKey, cosmosSig, minaSig, err := signValidatorRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_VALIDATOR,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	return cosmosPriv, minaPubKey, newMinaPriv
}

func TestValidatorUpdateKeysSuccess(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv, prevMinaPubKey, newMinaPriv := registerValidatorKeysForUpdateTest(t, f, ms)

	creator, cosmosPubKey, newMinaPubKey, cosmosSig, newMinaSig, err := signValidatorRegistration(cosmosPriv, newMinaPriv)
	require.NoError(t, err)

	resp, err := ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPubKey,
		NewMinaPublicKey:  newMinaPubKey,
		CosmosSignature:   cosmosSig,
		NewMinaSignature:  newMinaSig,
		ActorType:         types.ActorType_VALIDATOR,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	exists, err := f.keeper.ValidatorMinaToCosmosHas(f.ctx, prevMinaPubKey)
	require.NoError(t, err)
	require.False(t, exists)

	exists, err = f.keeper.ValidatorMinaToCosmosHas(f.ctx, newMinaPubKey)
	require.NoError(t, err)
	require.True(t, exists)

	minaPubKey, err := f.keeper.ValidatorGetCosmosToMina(f.ctx, cosmosPubKey)
	require.NoError(t, err)
	require.Equal(t, newMinaPubKey, minaPubKey)

	storedCosmosPubKey, err := f.keeper.ValidatorGetMinaToCosmos(f.ctx, newMinaPubKey)
	require.NoError(t, err)
	require.Equal(t, cosmosPubKey, storedCosmosPubKey)
}

func TestValidatorUpdateKeysNotRegistered(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateValidatorCosmosPrivKey()

	prevMinaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	newMinaPriv, err := generateMinaSecondaryKeyPair(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	prevMinaPk, err := prevMinaPriv.ToPublicKey()
	require.NoError(t, err)

	creator, _, newMinaPubKey, cosmosSig, newMinaSig, err := signValidatorRegistration(cosmosPriv, newMinaPriv)
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPk.Bytes(),
		NewMinaPublicKey:  newMinaPubKey,
		CosmosSignature:   cosmosSig,
		NewMinaSignature:  newMinaSig,
		ActorType:         types.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, types.ErrValidatorNotRegistered)
}

func TestValidatorUpdateKeysMissingCosmosToMinaMapping(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateValidatorCosmosPrivKey()

	prevMinaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	newMinaPriv, err := generateMinaSecondaryKeyPair(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	prevMinaPk, err := prevMinaPriv.ToPublicKey()
	require.NoError(t, err)

	creator, _, newMinaPubKey, cosmosSig, newMinaSig, err := signValidatorRegistration(cosmosPriv, newMinaPriv)
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPk.Bytes(),
		NewMinaPublicKey:  newMinaPubKey,
		CosmosSignature:   cosmosSig,
		NewMinaSignature:  newMinaSig,
		ActorType:         types.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, types.ErrValidatorNotRegistered)
}

func TestValidatorUpdateKeysInvalidCreatorAddress(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, prevMinaPubKey, newMinaPriv := registerValidatorKeysForUpdateTest(t, f, ms)

	_, _, newMinaPubKey, cosmosSig, newMinaSig, err := signValidatorRegistration(generateValidatorCosmosPrivKey(), newMinaPriv)
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           "creator",
		PrevMinaPublicKey: prevMinaPubKey,
		NewMinaPublicKey:  newMinaPubKey,
		CosmosSignature:   cosmosSig,
		NewMinaSignature:  newMinaSig,
		ActorType:         types.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, types.ErrInvalidCreatorAddress)
}

func TestValidatorUpdateKeysInvalidSignature(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv, prevMinaPubKey, newMinaPriv := registerValidatorKeysForUpdateTest(t, f, ms)

	creator, _, newMinaPubKey, _, newMinaSig, err := signValidatorRegistration(cosmosPriv, newMinaPriv)
	require.NoError(t, err)

	wrongCosmosSig, err := generateValidatorCosmosPrivKey().Sign(newMinaPubKey)
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPubKey,
		NewMinaPublicKey:  newMinaPubKey,
		CosmosSignature:   wrongCosmosSig,
		NewMinaSignature:  newMinaSig,
		ActorType:         types.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, types.ErrInvalidSignature)
}

func TestValidatorUpdateKeysInsertSecondaryKeysFail(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	firstCosmosPriv, prevMinaPublicKey, targetMinaPriv := registerValidatorKeysForUpdateTest(t, f, ms)

	secondCosmosPriv := generateValidatorCosmosPrivKey()

	creator, cosmosPubKey, targetMinaPublicKey, cosmosSig, targetMinaSig, err := signValidatorRegistration(secondCosmosPriv, targetMinaPriv)
	require.NoError(t, err)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   targetMinaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   targetMinaPublicKey,
		ActorType:       types.ActorType_VALIDATOR,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	creator, _, targetMinaPublicKey, cosmosSig, targetMinaSig, err = signValidatorRegistration(firstCosmosPriv, targetMinaPriv)
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPublicKey,
		NewMinaPublicKey:  targetMinaPublicKey,
		CosmosSignature:   cosmosSig,
		NewMinaSignature:  targetMinaSig,
		ActorType:         types.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, types.ErrValidatorSecondaryKeyExists)
}
