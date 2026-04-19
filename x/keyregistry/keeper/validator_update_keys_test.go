package keeper_test

import (
	"crypto/rand"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

func generateValidatorKeyPair() (string, []byte, []byte, error) {
	cosmosPrivKey := ed25519.GenPrivKey()
	cosmosPubKey := cosmosPrivKey.PubKey()

	var minaSeed [32]byte
	_, err := rand.Read(minaSeed[:])
	if err != nil {
		return "", nil, nil, err
	}

	minaPubKey, err := keys.NewPrivateKeyFromBytes(minaSeed).ToPublicKey().Marshal()
	if err != nil {
		return "", nil, nil, err
	}

	return sdk.ConsAddress(cosmosPubKey.Address()).String(), cosmosPubKey.Bytes(), minaPubKey, nil
}

func registerValidatorKeysForUpdateTest(t *testing.T, f *fixture, ms types.MsgServer) (string, []byte, []byte) {
	t.Helper()

	creator, cosmosPubKey, minaPubKey, err := generateValidatorKeyPair()
	require.NoError(t, err)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   cosmosPubKey,
		MinaAddress:     minaPubKey,
		ActorType:       types.ActorType_VALIDATOR,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	return creator, cosmosPubKey, minaPubKey
}

func TestValidatorUpdateKeysSuccess(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	creator, cosmosPubKey, prevMinaPubKey := registerValidatorKeysForUpdateTest(t, f, ms)
	_, _, newMinaPubKey, err := generateValidatorKeyPair()
	require.NoError(t, err)

	resp, err := ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPubKey,
		NewMinaPublicKey:  newMinaPubKey,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
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

	creator, _, prevMinaPubKey, err := generateValidatorKeyPair()
	require.NoError(t, err)
	_, _, newMinaPubKey, err := generateValidatorKeyPair()
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPubKey,
		NewMinaPublicKey:  newMinaPubKey,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, types.ErrValidatorNotRegistered)
}

func TestValidatorUpdateKeysMissingCosmosToMinaMapping(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	creator, cosmosPubKey, prevMinaPubKey, err := generateValidatorKeyPair()
	require.NoError(t, err)
	_, _, newMinaPubKey, err := generateValidatorKeyPair()
	require.NoError(t, err)

	err = f.keeper.ValidatorSetMinaToCosmos(f.ctx, prevMinaPubKey, cosmosPubKey)
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPubKey,
		NewMinaPublicKey:  newMinaPubKey,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, types.ErrValidatorNotRegistered)
}

func TestValidatorUpdateKeysInvalidCreatorAddress(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, _, prevMinaPubKey, err := generateValidatorKeyPair()
	require.NoError(t, err)
	_, _, newMinaPubKey, err := generateValidatorKeyPair()
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           "creator",
		PrevMinaPublicKey: prevMinaPubKey,
		NewMinaPublicKey:  newMinaPubKey,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, types.ErrInvalidCreatorAddres)
}

func TestValidatorUpdateKeysInvalidSigner(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, _, prevMinaPubKey := registerValidatorKeysForUpdateTest(t, f, ms)
	secondaryCreator, _, newMinaPubKey, err := generateValidatorKeyPair()
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           secondaryCreator,
		PrevMinaPublicKey: prevMinaPubKey,
		NewMinaPublicKey:  newMinaPubKey,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, types.ErrInvalidSigner)
}

func TestValidatorUpdateKeysInvalidSignature(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	creator, _, prevMinaPubKey := registerValidatorKeysForUpdateTest(t, f, ms)
	_, _, newMinaPubKey, err := generateValidatorKeyPair()
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPubKey,
		NewMinaPublicKey:  newMinaPubKey,
		CosmosSignature:   []byte("cosmosSig"),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_VALIDATOR,
	})
	require.NoError(t, err)
}

func TestValidatorUpdateKeysInsertSecondaryKeysFail(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	creator, _, prevMinaPubKey := registerValidatorKeysForUpdateTest(t, f, ms)
	_, _, newMinaPubKey := registerValidatorKeysForUpdateTest(t, f, ms)

	_, err := ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creator,
		PrevMinaPublicKey: prevMinaPubKey,
		NewMinaPublicKey:  newMinaPubKey,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_VALIDATOR,
	})
	require.ErrorIs(t, err, types.ErrValidatorSecondaryKeyExists)
}
