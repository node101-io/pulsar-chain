package keeper_test

import (
	"testing"

	"github.com/cometbft/cometbft/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

func registerValidatorKeysForUpdateTest(t *testing.T, f *fixture, ms types.MsgServer) (crypto.PubKey, []byte, []byte) {
	t.Helper()

	cosmosPublicKey, minaPublicKey, minaSecondaryPublicKey, err := generateValidatorPublicKeys()
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
		ActorType:       types.ActorType_VALIDATOR,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	return cosmosPublicKey, minaPublicKey, minaSecondaryPublicKey
}

func TestValidatorUpdateKeysSuccess(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPublicKey, prevMinaPubKey, newMinaPubKey := registerValidatorKeysForUpdateTest(t, f, ms)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	resp, err := ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creatorAddr.String(),
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

	minaPubKey, err := f.keeper.ValidatorGetCosmosToMina(f.ctx, cosmosPublicKey.Bytes())
	require.NoError(t, err)
	require.Equal(t, newMinaPubKey, minaPubKey)

	storedCosmosPubKey, err := f.keeper.ValidatorGetMinaToCosmos(f.ctx, newMinaPubKey)
	require.NoError(t, err)
	require.Equal(t, cosmosPublicKey.Bytes(), storedCosmosPubKey)
}

func TestValidatorUpdateKeysNotRegistered(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPublicKey, prevMinaPubKey, newMinaPubKey, err := generateValidatorPublicKeys()
	require.NoError(t, err)
	require.NotNil(t, cosmosPublicKey)
	require.NotNil(t, prevMinaPubKey)
	require.NotNil(t, newMinaPubKey)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creatorAddr.String(),
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

	cosmosPublicKey, prevMinaPubKey, newMinaPubKey, err := generateValidatorPublicKeys()
	require.NoError(t, err)
	require.NotNil(t, cosmosPublicKey)
	require.NotNil(t, prevMinaPubKey)
	require.NotNil(t, newMinaPubKey)

	err = f.keeper.ValidatorSetMinaToCosmos(f.ctx, prevMinaPubKey, cosmosPublicKey.Bytes())
	require.NoError(t, err)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creatorAddr.String(),
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

	_, prevMinaPubKey, newMinaPubKey, err := generateValidatorPublicKeys()

	require.NoError(t, err)
	require.NotNil(t, prevMinaPubKey)
	require.NotNil(t, newMinaPubKey)

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

func TestValidatorUpdateKeysInvalidSignature(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPublicKey, prevMinaPubKey, newMinaPublicKey := registerValidatorKeysForUpdateTest(t, f, ms)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	_, err := ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           creatorAddr.String(),
		PrevMinaPublicKey: prevMinaPubKey,
		NewMinaPublicKey:  newMinaPublicKey,
		CosmosSignature:   []byte("cosmosSig"),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_VALIDATOR,
	})
	require.NoError(t, err)
}

func TestValidatorUpdateKeysInsertSecondaryKeysFail(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	firstCosmosPublicKey, prevMinaPublicKey, _ := registerValidatorKeysForUpdateTest(t, f, ms)
	secondCosmosPublicKey, _, targetMinaPublicKey, err := generateValidatorPublicKeys()
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
		ActorType:       types.ActorType_VALIDATOR,
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
		ActorType:         types.ActorType_VALIDATOR,
	})

	require.ErrorIs(t, err, types.ErrValidatorSecondaryKeyExists)
}
