package keeper_test

import (
	"bytes"
	"testing"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	cometed25519 "github.com/cometbft/cometbft/crypto/ed25519"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/node101-io/mina-signer-go/publickey"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

var MinaPriv = []byte("7olA5Knafb5E2hJoWFzD+oamtyXIXXUZmYG9+pBMjTGIjqZTVLNGbE7DQ3Zq5YL5NMW31UMMMGgNCeEk+gyzRA==")
var MinaSecondaryPriv = []byte("0GUKibsJSZwgiU7k4cXQQWb2QKEP9/iRFATJEUqf2Pc+GxciLMKRQGTIcInKsTzV09rjDsLmZiBl9Up71bvV6g==")

func malformedMinaPublicKey() []byte {
	return bytes.Repeat([]byte{0xff}, publickey.Size())
}

func malformedMinaSignature() []byte {
	return []byte("bad-mina-signature")
}

// testMinaNetworkID mirrors app.toml's mina.network_id. Registration
// signatures are verified under it, so tests must mint keys with the same
// domain the keeper is configured for.
const testMinaNetworkID = mina.TestNet

func generateMinaKey(actorType types.ActorType) (*privatekey.PrivateKey, error) {
	minaPrivKey, err := privatekey.NewPrivateKeyFromBytes([32]byte(MinaPriv), testMinaNetworkID)
	if err != nil {
		return nil, err
	}
	return minaPrivKey, nil
}

func generateMinaSecondaryKeyPair(actorType types.ActorType) (*privatekey.PrivateKey, error) {
	minaSecondaryPrivKey, err := privatekey.NewPrivateKeyFromBytes([32]byte(MinaSecondaryPriv), testMinaNetworkID)
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

	challenge, err := types.RegistrationChallenge(types.ActorType_USER, cosmosPubKey.Bytes())
	if err != nil {
		return "", nil, nil, nil, nil, err
	}

	minaSig, err := minaPriv.SignFieldElement(challenge)
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

	challenge, err := types.RegistrationChallenge(types.ActorType_VALIDATOR, cosmosPubKey.Bytes())
	if err != nil {
		return "", nil, nil, nil, nil, err
	}

	minaSig, err := minaPriv.SignFieldElement(challenge)
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

	challenge, err := types.RegistrationChallenge(types.ActorType_USER, cosmosPubKey.Bytes())
	require.NoError(t, err)

	minaSig, err := minaPriv.SignFieldElement(challenge)
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

// TestUserInvalidCreatorAddress verifies that RegisterKeys fails with ErrInvalidCreatorAddress
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

// TestUserInvalidSigner verifies that RegisterKeys fails with ErrInvalidCreatorAddress
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

func TestUserMalformedMinaSignature(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateUserCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)

	creator, cosmosPubKey, minaPubKey, cosmosSig, _, err := signUserRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   malformedMinaSignature(),
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidSignature)
}

func TestUserRegisterKeysMalformedMinaPublicKey(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateUserCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)

	creator, cosmosPubKey, _, cosmosSig, minaSig, err := signUserRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   malformedMinaPublicKey(),
		ActorType:       types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidPublicKey)
}

// TestUserRegisterKeysDuplicateCosmosKey verifies that reusing a registered
// Cosmos key with a different Mina key fails.
func TestUserRegisterKeysDuplicateCosmosKey(t *testing.T) {

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

// TestUserRegisterKeysDuplicateMinaKey verifies that reusing a registered Mina
// key with a different Cosmos key fails.
func TestUserRegisterKeysDuplicateMinaKey(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	firstCosmosPriv := generateUserCosmosPrivKey()
	secondCosmosPriv := generateUserCosmosPrivKey()

	minaPriv, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)

	creator, cosmosPubKey, minaPubKey, cosmosSig, minaSig, err := signUserRegistration(firstCosmosPriv, minaPriv)
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

	creator, cosmosPubKey, minaPubKey, cosmosSig, minaSig, err = signUserRegistration(secondCosmosPriv, minaPriv)
	require.NoError(t, err)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrUserSecondaryKeyExists)
}
