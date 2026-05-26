package keeper_test

import (
	"testing"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	cometed25519 "github.com/cometbft/cometbft/crypto/ed25519"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

var MinaPriv = []byte("7olA5Knafb5E2hJoWFzD+oamtyXIXXUZmYG9+pBMjTGIjqZTVLNGbE7DQ3Zq5YL5NMW31UMMMGgNCeEk+gyzRA==")
var MinaSecondaryPriv = []byte("0GUKibsJSZwgiU7k4cXQQWb2QKEP9/iRFATJEUqf2Pc+GxciLMKRQGTIcInKsTzV09rjDsLmZiBl9Up71bvV6g==")

func generateMinaKey(actorType types.ActorType) (*privatekey.PrivateKey, error) {
	minaPrivKey, err := privatekey.NewPrivateKeyFromBytes([32]byte(MinaPriv), mina.NetworkID(actorType.String()))
	if err != nil {
		return nil, err
	}
	return minaPrivKey, nil
}

func generateMinaSecondaryKeyPair(actorType types.ActorType) (*privatekey.PrivateKey, error) {
	minaSecondaryPrivKey, err := privatekey.NewPrivateKeyFromBytes([32]byte(MinaSecondaryPriv), mina.NetworkID(actorType.String()))
	if err != nil {
		return nil, err
	}

	return minaSecondaryPrivKey, nil
}

func generateUserCosmosPrivKey() secp256k1.PrivKey {
	return secp256k1.GenPrivKey()
}

func generateValidatorCosmosPrivKey() cometed25519.PrivKey {
	return cometed25519.GenPrivKey()
}

func signUserRegistration(cosmosPriv secp256k1.PrivKey, minaPriv *privatekey.PrivateKey) (string, []byte, []byte, []byte, []byte, error) {
	cosmosPubKey := cosmosPriv.PubKey()

	minaSig, err := minaPriv.SignBytes(cosmosPubKey.Bytes())
	if err != nil {
		return "", nil, nil, nil, nil, err
	}

	minaPk, err := minaPriv.ToPublicKey()
	if err != nil {
		return "", nil, nil, nil, nil, err
	}

	creatorAddr := sdk.AccAddress(cosmosPubKey.Address())

	cosmosSig, err := cosmosPriv.Sign(minaPk.Bytes())
	if err != nil {
		return "", nil, nil, nil, nil, err
	}

	return creatorAddr.String(), cosmosPubKey.Bytes(), minaPk.Bytes(), cosmosSig, minaSig.Bytes(), nil
}

func signValidatorRegistration(cosmosPriv cometed25519.PrivKey, minaPriv *privatekey.PrivateKey) (string, []byte, []byte, []byte, []byte, error) {
	cosmosPubKey := cosmosPriv.PubKey()

	minaSig, err := minaPriv.SignBytes(cosmosPubKey.Bytes())
	if err != nil {
		return "", nil, nil, nil, nil, err
	}

	minaPk, err := minaPriv.ToPublicKey()
	if err != nil {
		return "", nil, nil, nil, nil, err
	}

	creatorAddr := sdk.AccAddress(cosmosPubKey.Address())

	cosmosSig, err := cosmosPriv.Sign(minaPk.Bytes())
	if err != nil {
		return "", nil, nil, nil, nil, err
	}

	return creatorAddr.String(), cosmosPubKey.Bytes(), minaPk.Bytes(), cosmosSig, minaSig.Bytes(), nil
}

// TestUserRegisterKeysFail verifies that RegisterKeys fails with ErrInvalidPublicKey
// when the provided key material does not satisfy the current user registration checks.
func TestUserRegisterKeysFail(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateUserCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)

	creator, _, minaPubKey, cosmosSig, minaSig, err := signUserRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: []byte("bad-cosmos-key"),
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidPublicKey)

}

// TestUserRegisterKeysSuccess verifies that RegisterKeys succeeds with valid inputs
// and ensures that both CosmosToMina and MinaToCosmos mappings are correctly stored.
func TestUserRegisterKeysSuccess(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateUserCosmosPrivKey()
	cosmosPubKey := cosmosPriv.PubKey()

	minaPriv, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)
	require.NotNil(t, minaPriv)

	minaSig, err := minaPriv.SignBytes(cosmosPubKey.Bytes())
	require.NoError(t, err)
	require.NotNil(t, minaSig)

	minaPk, err := minaPriv.ToPublicKey()
	require.NoError(t, err)
	require.NotNil(t, minaPk)

	creatorAddr := sdk.AccAddress(cosmosPubKey.Address())
	require.NotNil(t, creatorAddr)

	cosmosSig, err := cosmosPriv.Sign(minaPk.Bytes())
	require.NoError(t, err)
	require.NotNil(t, cosmosSig)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig.Bytes(),
		CosmosPublicKey: cosmosPubKey.Bytes(),
		MinaPublicKey:   minaPk.Bytes(),
		ActorType:       types.ActorType_USER,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
}

// TestUserInvalidCreatorAddress verifies that RegisterKeys fails with ErrInvalidCreatorAddres
// when the creator field is not a valid bech32 address.
func TestUserInvalidCreatorAddress(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateUserCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)

	_, cosmosPubKey, minaPubKey, cosmosSig, minaSig, err := signUserRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         "creator",
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidCreatorAddress)
}

// TestUserInvalidSigner verifies that RegisterKeys fails with ErrInvalidSigner
// when the creator address bytes do not match the provided Cosmos-side value.
func TestUserInvalidSigner(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateUserCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)

	_, cosmosPubKey, minaPubKey, cosmosSig, minaSig, err := signUserRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	secondaryCreator := sdk.AccAddress(generateUserCosmosPrivKey().PubKey().Address()).String()

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         secondaryCreator,
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidCreatorAddress)
}

func TestUserInvalidSignature(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateUserCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)

	creator, cosmosPubKey, minaPubKey, _, minaSig, err := signUserRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	wrongCosmosSig, err := generateUserCosmosPrivKey().Sign(minaPubKey)
	require.NoError(t, err)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: wrongCosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidSignature)
}

// TestUserInsertSecondaryKeysFail verifies that registering the same key pair twice
// fails with ErrSecondaryKeyExists on the second attempt.
func TestUserInsertSecondaryKeysFail(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateUserCosmosPrivKey()

	minaPriv, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)

	secondaryMinaPriv, err := generateMinaSecondaryKeyPair(types.ActorType_USER)
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

	creator, cosmosPubKey, secondaryMinaPubKey, cosmosSig, secondaryMinaSig, err := signUserRegistration(cosmosPriv, secondaryMinaPriv)
	require.NoError(t, err)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   secondaryMinaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   secondaryMinaPubKey,
		ActorType:       types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrUserSecondaryKeyExists)
}
