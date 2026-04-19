package keeper_test

import (
	"crypto/rand"
	"testing"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

func generateUserAddressPair() (sdk.AccAddress, []byte, error) {
	cosmosPrivKey := secp256k1.GenPrivKey()
	cosmosAddr := sdk.AccAddress(cosmosPrivKey.PubKey().Address())

	var minaSeed [32]byte
	_, err := rand.Read(minaSeed[:])
	if err != nil {
		return nil, nil, err
	}

	minaAddress, err := keys.NewPrivateKeyFromBytes(minaSeed).ToPublicKey().ToAddress()
	if err != nil {
		return nil, nil, err
	}

	return cosmosAddr, []byte(minaAddress), nil
}

func registerUserKeysForUpdateTest(t *testing.T, f *fixture, ms types.MsgServer) (sdk.AccAddress, []byte) {
	t.Helper()

	cosmosAddr, minaAddr, err := generateUserAddressPair()
	require.NoError(t, err)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         cosmosAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   cosmosAddr.Bytes(),
		MinaAddress:     minaAddr,
		ActorType:       types.ActorType_USER,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	return cosmosAddr, minaAddr
}

func TestUserUpdateKeysSuccess(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosAddr, prevMinaAddr := registerUserKeysForUpdateTest(t, f, ms)
	_, newMinaAddr, err := generateUserAddressPair()
	require.NoError(t, err)

	resp, err := ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           cosmosAddr.String(),
		PrevMinaPublicKey: prevMinaAddr,
		NewMinaPublicKey:  newMinaAddr,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_USER,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	exists, err := f.keeper.UserMinaToCosmosHas(f.ctx, prevMinaAddr)
	require.NoError(t, err)
	require.False(t, exists)

	exists, err = f.keeper.UserMinaToCosmosHas(f.ctx, newMinaAddr)
	require.NoError(t, err)
	require.True(t, exists)

	minaAddr, err := f.keeper.UserGetCosmosToMina(f.ctx, cosmosAddr.Bytes())
	require.NoError(t, err)
	require.Equal(t, newMinaAddr, minaAddr)

	storedCosmosAddr, err := f.keeper.UserGetMinaToCosmos(f.ctx, newMinaAddr)
	require.NoError(t, err)
	require.Equal(t, cosmosAddr.Bytes(), storedCosmosAddr)
}

func TestUserUpdateKeysNotRegistered(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosAddr, prevMinaAddr, err := generateUserAddressPair()
	require.NoError(t, err)
	_, newMinaAddr, err := generateUserAddressPair()
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           cosmosAddr.String(),
		PrevMinaPublicKey: prevMinaAddr,
		NewMinaPublicKey:  newMinaAddr,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrUserNotRegistered)
}

func TestUserUpdateKeysInvalidCreatorAddress(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, prevMinaAddr, err := generateUserAddressPair()
	require.NoError(t, err)
	_, newMinaAddr, err := generateUserAddressPair()
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           "creator",
		PrevMinaPublicKey: prevMinaAddr,
		NewMinaPublicKey:  newMinaAddr,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidCreatorAddres)
}

func TestUserUpdateKeysInvalidSigner(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, prevMinaAddr := registerUserKeysForUpdateTest(t, f, ms)
	secondaryAddr, newMinaAddr, err := generateUserAddressPair()
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           secondaryAddr.String(),
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

	cosmosAddr, prevMinaAddr := registerUserKeysForUpdateTest(t, f, ms)
	_, newMinaAddr, err := generateUserAddressPair()
	require.NoError(t, err)

	_, err = ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           cosmosAddr.String(),
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

	cosmosAddr, prevMinaAddr := registerUserKeysForUpdateTest(t, f, ms)
	_, newMinaAddr := registerUserKeysForUpdateTest(t, f, ms)

	_, err := ms.UpdateKeys(f.ctx, &types.MsgUpdateKeys{
		Creator:           cosmosAddr.String(),
		PrevMinaPublicKey: prevMinaAddr,
		NewMinaPublicKey:  newMinaAddr,
		CosmosSignature:   []byte(mockCosmosSignature),
		NewMinaSignature:  []byte(mockMinaSignature),
		ActorType:         types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrUserSecondaryKeyExists)
}
