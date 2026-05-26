package keeper_test

import (
	"testing"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/node101-io/mina-signer-go/privatekey"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

func registerUserKeysForUpdateTest(t *testing.T, f *fixture, ms types.MsgServer) (secp256k1.PrivKey, []byte, *privatekey.PrivateKey) {
	t.Helper()

	cosmosPriv := generateUserCosmosPrivKey()

	minaPriv, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)

	newMinaPriv, err := generateMinaSecondaryKeyPair(types.ActorType_USER)
	require.NoError(t, err)

	creator, cosmosPubKey, minaPubKey, cosmosSig, minaSig, err := signUserRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	return cosmosPriv, minaPubKey, newMinaPriv
}

func TestUserUpdateKeysSuccess(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv, prevMinaPublicKey, newMinaPriv := registerUserKeysForUpdateTest(t, f, ms)

	creator, cosmosPubKey, newMinaPublicKey, cosmosSig, newMinaSig, err := signUserRegistration(cosmosPriv, newMinaPriv)
	require.NoError(t, err)

	resp, err := ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPublicKey,
		NewMinaPublicKey:  newMinaPublicKey,
		CosmosSignature:   cosmosSig,
		NewMinaSignature:  newMinaSig,
		ActorType:         types.ActorType_USER,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	exists, err := f.keeper.UserMinaToCosmosHas(f.ctx, prevMinaPublicKey)
	require.NoError(t, err)
	require.False(t, exists)

	exists, err = f.keeper.UserMinaToCosmosHas(f.ctx, newMinaPublicKey)
	require.NoError(t, err)
	require.True(t, exists)

	minaAddr, err := f.keeper.UserGetCosmosToMina(f.ctx, cosmosPubKey)
	require.NoError(t, err)
	require.Equal(t, newMinaPublicKey, minaAddr)

	storedCosmosAddr, err := f.keeper.UserGetMinaToCosmos(f.ctx, newMinaPublicKey)
	require.NoError(t, err)
	require.Equal(t, cosmosPubKey, storedCosmosAddr)
}

func TestUserUpdateKeysNotRegistered(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateUserCosmosPrivKey()

	prevMinaPriv, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)

	newMinaPriv, err := generateMinaSecondaryKeyPair(types.ActorType_USER)
	require.NoError(t, err)

	prevMinaPk, err := prevMinaPriv.ToPublicKey()
	require.NoError(t, err)

	creator, _, newMinaPublicKey, cosmosSig, newMinaSig, err := signUserRegistration(cosmosPriv, newMinaPriv)
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPk.Bytes(),
		NewMinaPublicKey:  newMinaPublicKey,
		CosmosSignature:   cosmosSig,
		NewMinaSignature:  newMinaSig,
		ActorType:         types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrUserNotRegistered)
}

func TestUserUpdateKeysInvalidCreatorAddress(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv, prevMinaPublicKey, newMinaPriv := registerUserKeysForUpdateTest(t, f, ms)

	_, _, newMinaPublicKey, cosmosSig, newMinaSig, err := signUserRegistration(cosmosPriv, newMinaPriv)
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           "creator",
		PrevMinaPublicKey: prevMinaPublicKey,
		NewMinaPublicKey:  newMinaPublicKey,
		CosmosSignature:   cosmosSig,
		NewMinaSignature:  newMinaSig,
		ActorType:         types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidCreatorAddress)
}

func TestUserUpdateKeysInvalidSigner(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv, prevMinaPublicKey, newMinaPriv := registerUserKeysForUpdateTest(t, f, ms)

	_, _, newMinaPublicKey, cosmosSig, newMinaSig, err := signUserRegistration(cosmosPriv, newMinaPriv)
	require.NoError(t, err)

	secondaryCreator := sdk.AccAddress(generateUserCosmosPrivKey().PubKey().Address()).String()

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           secondaryCreator,
		PrevMinaPublicKey: prevMinaPublicKey,
		NewMinaPublicKey:  newMinaPublicKey,
		CosmosSignature:   cosmosSig,
		NewMinaSignature:  newMinaSig,
		ActorType:         types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidCreatorAddress)
}

func TestUserUpdateKeysInvalidSignature(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv, prevMinaPublicKey, newMinaPriv := registerUserKeysForUpdateTest(t, f, ms)

	creator, _, newMinaPublicKey, _, newMinaSig, err := signUserRegistration(cosmosPriv, newMinaPriv)
	require.NoError(t, err)

	wrongCosmosSig, err := generateUserCosmosPrivKey().Sign(newMinaPublicKey)
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPublicKey,
		NewMinaPublicKey:  newMinaPublicKey,
		CosmosSignature:   wrongCosmosSig,
		NewMinaSignature:  newMinaSig,
		ActorType:         types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidSignature)
}

func TestUserUpdateKeysInsertSecondaryKeysFail(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	firstCosmosPriv, prevMinaPublicKey, targetMinaPriv := registerUserKeysForUpdateTest(t, f, ms)

	secondCosmosPriv := generateUserCosmosPrivKey()

	creator, cosmosPubKey, targetMinaPublicKey, cosmosSig, targetMinaSig, err := signUserRegistration(secondCosmosPriv, targetMinaPriv)
	require.NoError(t, err)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   targetMinaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   targetMinaPublicKey,
		ActorType:       types.ActorType_USER,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	creator, _, targetMinaPublicKey, cosmosSig, targetMinaSig, err = signUserRegistration(firstCosmosPriv, targetMinaPriv)
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPublicKey,
		NewMinaPublicKey:  targetMinaPublicKey,
		CosmosSignature:   cosmosSig,
		NewMinaSignature:  targetMinaSig,
		ActorType:         types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrUserSecondaryKeyExists)
}
