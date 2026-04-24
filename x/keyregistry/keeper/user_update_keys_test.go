package keeper_test

import (
	"testing"

	"github.com/cometbft/cometbft/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

var MinaSecondaryPriv = []byte("0GUKibsJSZwgiU7k4cXQQWb2QKEP9/iRFATJEUqf2Pc+GxciLMKRQGTIcInKsTzV09rjDsLmZiBl9Up71bvV6g==")

func registerUserKeysForUpdateTest(t *testing.T, f *fixture, ms types.MsgServer) (crypto.PubKey, []byte, []byte) {
	t.Helper()

	cosmosPublicKey, minaPublicKey, minaSecondaryPublicKey, err := generateUserPublicKeys()
	require.NotNil(t, cosmosPublicKey)
	require.NotNil(t, minaPublicKey)
	require.NotNil(t, minaSecondaryPublicKey)

	require.NoError(t, err)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPublicKey.Bytes(),
		MinaPublicKey:   minaPublicKey,
		ActorType:       types.ActorType_USER,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	return cosmosPublicKey, minaPublicKey, minaSecondaryPublicKey
}

func TestUserUpdateKeysSuccess(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPublicKey, prevMinaPublicKey, newMinaPublicKey := registerUserKeysForUpdateTest(t, f, ms)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	resp, err := ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creatorAddr.String(),
		PrevMinaPublicKey: prevMinaPublicKey,
		NewMinaPublicKey:  newMinaPublicKey,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
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

	minaAddr, err := f.keeper.UserGetCosmosToMina(f.ctx, cosmosPublicKey.Bytes())
	require.NoError(t, err)
	require.Equal(t, newMinaPublicKey, minaAddr)

	storedCosmosAddr, err := f.keeper.UserGetMinaToCosmos(f.ctx, newMinaPublicKey)
	require.NoError(t, err)
	require.Equal(t, cosmosPublicKey.Bytes(), storedCosmosAddr)
}

func TestUserUpdateKeysNotRegistered(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPublicKey, prevMinaPublicKey, minaSecondaryPublicKey, err := generateUserPublicKeys()
	require.NoError(t, err)
	require.NotNil(t, cosmosPublicKey)
	require.NotNil(t, prevMinaPublicKey)
	require.NotNil(t, minaSecondaryPublicKey)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creatorAddr.String(),
		PrevMinaPublicKey: prevMinaPublicKey,
		NewMinaPublicKey:  minaSecondaryPublicKey,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrUserNotRegistered)
}

func TestUserUpdateKeysInvalidCreatorAddress(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, prevMinaPublicKey, newMinaPublicKey := registerUserKeysForUpdateTest(t, f, ms)

	_, err := ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           "creator",
		PrevMinaPublicKey: prevMinaPublicKey,
		NewMinaPublicKey:  newMinaPublicKey,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_USER,
	})

	require.ErrorIs(t, err, types.ErrInvalidCreatorAddress)
}

func TestUserUpdateKeysInvalidSigner(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, prevMinaAddr, _ := registerUserKeysForUpdateTest(t, f, ms)

	secondaryAddr, newMinaAddr, _, err := generateUserPublicKeys()
	require.NoError(t, err)
	require.NotNil(t, secondaryAddr)
	require.NotNil(t, newMinaAddr)

	creatorAddr := sdk.AccAddress(secondaryAddr.Address())
	require.NotNil(t, creatorAddr)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creatorAddr.String(),
		PrevMinaPublicKey: prevMinaAddr,
		NewMinaPublicKey:  newMinaAddr,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidSigner)
}

func TestUserUpdateKeysInvalidSignature(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPublicKey, prevMinaAddr, newMinaAddr := registerUserKeysForUpdateTest(t, f, ms)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())

	_, err := ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creatorAddr.String(),
		PrevMinaPublicKey: prevMinaAddr,
		NewMinaPublicKey:  newMinaAddr,
		CosmosSignature:   []byte("cosmosSig"),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_USER,
	})
	require.NoError(t, err)
}

func TestUserUpdateKeysInsertSecondaryKeysFail(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	firstCosmosPublicKey, prevMinaPublicKey, _ := registerUserKeysForUpdateTest(t, f, ms)
	secondCosmosPublicKey, _, targetMinaPublicKey, err := generateUserPublicKeys()
	require.NoError(t, err)
	require.NotNil(t, secondCosmosPublicKey)
	require.NotNil(t, targetMinaPublicKey)

	creatorAddr := sdk.AccAddress(secondCosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: secondCosmosPublicKey.Bytes(),
		MinaPublicKey:   targetMinaPublicKey,
		ActorType:       types.ActorType_USER,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	creatorAddr = sdk.AccAddress(firstCosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creatorAddr.String(),
		PrevMinaPublicKey: prevMinaPublicKey,
		NewMinaPublicKey:  targetMinaPublicKey,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_USER,
	})

	require.ErrorIs(t, err, types.ErrUserSecondaryKeyExists)
}
