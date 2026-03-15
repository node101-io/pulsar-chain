package keeper_test

import (
	"fmt"
	"testing"

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
var mockCosmosSignature = "cosmosSig"
var mockMinaSignature = "minaSig"

var MinaPriv = []byte("7olA5Knafb5E2hJoWFzD+oamtyXIXXUZmYG9+pBMjTGIjqZTVLNGbE7DQ3Zq5YL5NMW31UMMMGgNCeEk+gyzRA==")

func generateAddress() (crypto.PubKey, []byte, error) {
	cosmosPrivKey := secp256k1.GenPrivKey()

	cosmosPubKey := cosmosPrivKey.PubKey()

	minaPrivKey := keys.NewPrivateKeyFromBytes([32]byte(MinaPriv))

	minaAddress, err := minaPrivKey.ToPublicKey().ToAddress()

	return cosmosPubKey, []byte(minaAddress), err
}

// TestUserRegisterKeysFail verifies that RegisterKeys fails with ErrInvalidPublicKey
// when the provided cosmos public key is not a valid compressed secp256k1 key (33 bytes).
func TestUserRegisterKeysFail(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	creatorAddr := sdk.AccAddress([]byte("pulsar"))
	_, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   CosmosPubKey,
		MinaAddress:     MinaPubKey,
		IsUser:          true,
	})
	require.ErrorIs(t, err, types.ErrInvalidAddress)
}

// TestUserRegisterKeysSuccess verifies that RegisterKeys succeeds with valid inputs
// and ensures that both CosmosToMina and MinaToCosmos mappings are correctly stored.
func TestUserRegisterKeysSuccess(t *testing.T) {

	cosmosAddr, minaAddr, err := generateAddress()
	require.NoError(t, err)

	addr := sdk.AccAddress(cosmosAddr.Address())

	fmt.Println("addr len", len(addr.Bytes()))
	fmt.Println("len mina addr", len(minaAddr))
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   addr.Bytes(),
		MinaAddress:     minaAddr,
		IsUser:          true,
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

	cosmosPubKey, minaPubKey, err := generatePublicKeys()
	require.NoError(t, err)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         "creator",
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   cosmosPubKey.Bytes(),
		MinaAddress:     minaPubKey,
		IsUser:          true,
	})
	require.ErrorIs(t, err, types.ErrInvalidCreatorAddres)
}

// TestUserInvalidSigner verifies that RegisterKeys fails with ErrInvalidSigner
// when the creator address does not match the address derived from the provided cosmos public key.
func TestUserInvalidSigner(t *testing.T) {

	cosmosPubKey, minaPubKey, err := generatePublicKeys()
	require.NoError(t, err)

	secondaryPriv := secp256k1.GenPrivKey()

	secondaryPublic := secondaryPriv.PubKey()

	addr := sdk.AccAddress(secondaryPublic.Address())

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   cosmosPubKey.Bytes(),
		MinaAddress:     minaPubKey,
		IsUser:          true,
	})

	require.ErrorIs(t, err, types.ErrInvalidSigner)

}

// TODO: Update require.NoError to require.ErrorIs once the VerifyCosmosSig and VerifyMinaSig is implemented
// TestUserInvalidSignature currently expects no error since signature verification is not yet implemented.
func TestUserInvalidSignature(t *testing.T) {

	cosmosPubKey, minaPubKey, err := generatePublicKeys()
	addr := sdk.AccAddress(cosmosPubKey.Address())

	require.NoError(t, err)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	invalidSig := "cosmosSig"

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: invalidSig,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   cosmosPubKey.Bytes(),
		MinaAddress:     minaPubKey,
		IsUser:          true,
	})

	require.NoError(t, err)

}

// TestUserInsertSecondaryKeysFail verifies that registering the same key pair twice
// fails with ErrSecondaryKeyExists on the second attempt.
func TestUserInsertSecondaryKeysFail(t *testing.T) {
	f := initFixture(t)

	cosmosPubKey, minaPubKey, err := generatePublicKeys()
	require.NoError(t, err)

	addr := sdk.AccAddress(cosmosPubKey.Address())

	ms := keeper.NewMsgServerImpl(f.keeper)

	// First registration should succeed.
	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   cosmosPubKey.Bytes(),
		MinaAddress:     minaPubKey,
		IsUser:          true,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Second registration with the same keys should fail.
	resp, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   cosmosPubKey.Bytes(),
		MinaAddress:     minaPubKey,
		IsUser:          true,
	})

	require.ErrorIs(t, err, types.ErrUserSecondaryKeyExists)
}
