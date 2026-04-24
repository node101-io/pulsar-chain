package keeper_test

import (
	"testing"

	cometed25519 "github.com/cometbft/cometbft/crypto/ed25519"

	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

// Mock signatures used across msg server tests.
// These will be replaced with real signatures once VerifyMinaSig and VerifyCosmosSig are implemented.
var mockCosmosSignature = []byte("cosmosSig")
var mockMinaSignature = []byte("minaSig")

var MinaPriv = []byte("7olA5Knafb5E2hJoWFzD+oamtyXIXXUZmYG9+pBMjTGIjqZTVLNGbE7DQ3Zq5YL5NMW31UMMMGgNCeEk+gyzRA==")

func generateUserPublicKeys() (crypto.PubKey, []byte, []byte, error) {

	cosmosPrivKey := secp256k1.GenPrivKey()
	cosmosPublicKey := cosmosPrivKey.PubKey()

	minaPrivKey := keys.NewPrivateKeyFromBytes([32]byte(MinaPriv))

	minaPublicKey, err := minaPrivKey.ToPublicKey().Marshal()
	if err != nil {
		return nil, nil, nil, err
	}

	minaSecondaryPrivKey := keys.NewPrivateKeyFromBytes([32]byte(MinaSecondaryPriv))

	minaSecondaryPublicKey, err := minaSecondaryPrivKey.ToPublicKey().Marshal()
	if err != nil {
		return nil, nil, nil, err
	}

	return cosmosPublicKey, minaPublicKey, minaSecondaryPublicKey, nil
}

func generateValidatorPublicKeys() (crypto.PubKey, []byte, []byte, error) {

	cosmosPrivKey := cometed25519.GenPrivKey()
	cosmosPublicKey := cosmosPrivKey.PubKey()

	minaPrivKey := keys.NewPrivateKeyFromBytes([32]byte(MinaPriv))

	minaPublicKey, err := minaPrivKey.ToPublicKey().Marshal()
	if err != nil {
		return nil, nil, nil, err
	}

	minaSecondaryPrivKey := keys.NewPrivateKeyFromBytes([32]byte(MinaSecondaryPriv))

	minaSecondaryPublicKey, err := minaSecondaryPrivKey.ToPublicKey().Marshal()
	if err != nil {
		return nil, nil, nil, err
	}

	return cosmosPublicKey, minaPublicKey, minaSecondaryPublicKey, nil
}

// TestUserRegisterKeysFail verifies that RegisterKeys fails with ErrInvalidPublicKey
// when the provided key material does not satisfy the current user registration checks.
func TestUserRegisterKeysFail(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	creatorAddr := sdk.AccAddress([]byte("pulsar"))

	_, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: CosmosPubKey,
		MinaPublicKey:   MinaPubKey,
		ActorType:       types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidPublicKey)
}

// TestUserRegisterKeysSuccess verifies that RegisterKeys succeeds with valid inputs
// and ensures that both CosmosToMina and MinaToCosmos mappings are correctly stored.
func TestUserRegisterKeysSuccess(t *testing.T) {

	cosmosPublicKey, minaPubKey, _, err := generateUserPublicKeys()
	require.NoError(t, err)
	require.NotNil(t, cosmosPublicKey)
	require.NotNil(t, minaPubKey)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPublicKey.Bytes(),
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	exists, err := f.keeper.UserCosmosToMinaHas(f.ctx, cosmosPublicKey.Bytes())
	require.NoError(t, err)
	require.Equal(t, exists, true)

	exists, err = f.keeper.UserMinaToCosmosHas(f.ctx, minaPubKey)
	require.NoError(t, err)
	require.Equal(t, exists, true)

}

// TestUserInvalidCreatorAddress verifies that RegisterKeys fails with ErrInvalidCreatorAddres
// when the creator field is not a valid bech32 address.
func TestUserInvalidCreatorAddress(t *testing.T) {

	cosmosPublicKey, minaPubKey, _, err := generateUserPublicKeys()

	require.NotNil(t, cosmosPublicKey)
	require.NotNil(t, minaPubKey)
	require.NoError(t, err)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         "creator",
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPublicKey.Bytes(),
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidCreatorAddress)
}

// TestUserInvalidSigner verifies that RegisterKeys fails with ErrInvalidSigner
// when the creator address bytes do not match the provided Cosmos-side value.
func TestUserInvalidSigner(t *testing.T) {

	cosmosPublicKey, minaPubKey, _, err := generateUserPublicKeys()
	require.NoError(t, err)
	require.NotNil(t, cosmosPublicKey)
	require.NotNil(t, minaPubKey)

	secondaryCosmosPublicKey, _, _, err := generateUserPublicKeys()
	require.NoError(t, err)
	require.NotNil(t, secondaryCosmosPublicKey)

	creatorAddr := sdk.AccAddress(secondaryCosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPublicKey.Bytes(),
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})

	require.ErrorIs(t, err, types.ErrInvalidSigner)

}

// TODO: Update require.NoError to require.ErrorIs once the VerifyCosmosSig and VerifyMinaSig is implemented
// TestUserInvalidSignature currently expects no error since signature verification is not yet implemented.
func TestUserInvalidSignature(t *testing.T) {

	cosmosPublicKey, minaPubKey, _, err := generateUserPublicKeys()

	require.NoError(t, err)
	require.NotNil(t, cosmosPublicKey)
	require.NotNil(t, minaPubKey)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	invalidSig := []byte("cosmosSig")

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: invalidSig,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPublicKey.Bytes(),
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})

	require.NoError(t, err)

}

// TestUserInsertSecondaryKeysFail verifies that registering the same key pair twice
// fails with ErrSecondaryKeyExists on the second attempt.
func TestUserInsertSecondaryKeysFail(t *testing.T) {
	f := initFixture(t)

	cosmosPublicKey, minaPubKey, minaSecondaryPublicKey, err := generateUserPublicKeys()
	require.NoError(t, err)
	require.NotNil(t, cosmosPublicKey)
	require.NotNil(t, minaPubKey)
	require.NotNil(t, minaSecondaryPublicKey)

	ms := keeper.NewMsgServerImpl(f.keeper)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	// First registration should succeed.
	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPublicKey.Bytes(),
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Second registration with the same keys should fail.
	resp, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPublicKey.Bytes(),
		MinaPublicKey:   minaSecondaryPublicKey,
		ActorType:       types.ActorType_USER,
	})

	require.ErrorIs(t, err, types.ErrUserSecondaryKeyExists)
}
