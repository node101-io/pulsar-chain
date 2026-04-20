package keeper_test

import (
	"crypto/rand"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

// Mock signatures used across msg server tests.
// These will be replaced with real signatures once VerifyMinaSig and VerifyCosmosSig are implemented.
var mockCosmosSignature = "cosmosSig"
var mockMinaSignature = "minaSig"

var MinaPriv = []byte("7olA5Knafb5E2hJoWFzD+oamtyXIXXUZmYG9+pBMjTGIjqZTVLNGbE7DQ3Zq5YL5NMW31UMMMGgNCeEk+gyzRA==")

func generateAddress() (sdk.AccAddress, []byte, error) {
	cosmosAddr := make(sdk.AccAddress, 32)
	_, err := rand.Read(cosmosAddr)
	if err != nil {
		return nil, nil, err
	}

	minaPrivKey := keys.NewPrivateKeyFromBytes([32]byte(MinaPriv))

	minaPublicKey, err := minaPrivKey.ToPublicKey().Marshal()

	return cosmosAddr, minaPublicKey, err
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

	cosmosAddr, minaAddr, err := generateAddress()
	require.NoError(t, err)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         cosmosAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosAddr.Bytes(),
		MinaPublicKey:   minaAddr,
		ActorType:       types.ActorType_USER,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	exists, err := f.keeper.UserCosmosToMinaHas(f.ctx, cosmosAddr.Bytes())
	require.NoError(t, err)
	require.Equal(t, exists, true)

	exists, err = f.keeper.UserMinaToCosmosHas(f.ctx, minaAddr)
	require.NoError(t, err)
	require.Equal(t, exists, true)

}

// TestUserInvalidCreatorAddress verifies that RegisterKeys fails with ErrInvalidCreatorAddres
// when the creator field is not a valid bech32 address.
func TestUserInvalidCreatorAddress(t *testing.T) {

	cosmosAddr, minaAddr, err := generateAddress()
	require.NoError(t, err)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         "creator",
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosAddr.Bytes(),
		MinaPublicKey:   minaAddr,
		ActorType:       types.ActorType_USER,
	})
	require.ErrorIs(t, err, types.ErrInvalidCreatorAddres)
}

// TestUserInvalidSigner verifies that RegisterKeys fails with ErrInvalidSigner
// when the creator address bytes do not match the provided Cosmos-side value.
func TestUserInvalidSigner(t *testing.T) {

	cosmosAddr, minaAddr, err := generateAddress()
	require.NoError(t, err)

	addr, _, err := generateAddress()
	require.NoError(t, err)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosAddr.Bytes(),
		MinaPublicKey:   minaAddr,
		ActorType:       types.ActorType_USER,
	})

	require.ErrorIs(t, err, types.ErrInvalidSigner)

}

// TODO: Update require.NoError to require.ErrorIs once the VerifyCosmosSig and VerifyMinaSig is implemented
// TestUserInvalidSignature currently expects no error since signature verification is not yet implemented.
func TestUserInvalidSignature(t *testing.T) {

	cosmosAddr, minaAddr, err := generateAddress()

	require.NoError(t, err)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	invalidSig := "cosmosSig"

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         cosmosAddr.String(),
		CosmosSignature: invalidSig,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosAddr.Bytes(),
		MinaPublicKey:   minaAddr,
		ActorType:       types.ActorType_USER,
	})

	require.NoError(t, err)

}

// TestUserInsertSecondaryKeysFail verifies that registering the same key pair twice
// fails with ErrSecondaryKeyExists on the second attempt.
func TestUserInsertSecondaryKeysFail(t *testing.T) {
	f := initFixture(t)

	cosmosAddr, minaAddr, err := generateAddress()
	require.NoError(t, err)

	ms := keeper.NewMsgServerImpl(f.keeper)

	// First registration should succeed.
	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         cosmosAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosAddr.Bytes(),
		MinaPublicKey:   minaAddr,
		ActorType:       types.ActorType_USER,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Second registration with the same keys should fail.
	resp, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         cosmosAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosAddr.Bytes(),
		MinaPublicKey:   minaAddr,
		ActorType:       types.ActorType_USER,
	})

	require.ErrorIs(t, err, types.ErrUserSecondaryKeyExists)
}
