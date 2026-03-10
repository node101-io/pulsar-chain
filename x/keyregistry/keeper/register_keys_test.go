package keeper_test

import (
	"testing"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

// Mock signatures used across msg server tests.
// These will be replaced with real signatures once VerifyMinaSig and VerifyCosmosSig are implemented.
var mockCosmosSignature = "cosmosSig"
var mockMinaSignature = "minaSig"

// TestRegisterKeysFail verifies that RegisterKeys fails with ErrInvalidPublicKey
// when the provided cosmos public key is not a valid compressed secp256k1 key (33 bytes).
func TestRegisterKeysFail(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	creatorAddr := sdk.AccAddress([]byte("pulsar"))
	_, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: CosmosPubKey,
		MinaPublicKey:   MinaPubKey,
	})
	require.ErrorIs(t, err, types.ErrInvalidPublicKey)
}

// TestRegisterKeysSuccess verifies that RegisterKeys succeeds with valid inputs
// and ensures that both CosmosToMina and MinaToCosmos mappings are correctly stored.
func TestRegisterKeysSuccess(t *testing.T) {

	priv := secp256k1.GenPrivKey()

	pub := priv.PubKey()

	addr := sdk.AccAddress(pub.Address())

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: pub.Bytes(),
		MinaPublicKey:   MinaPubKey,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	exists, err := f.keeper.CosmosToMinaHas(f.ctx, pub.Bytes())
	require.NoError(t, err)
	require.Equal(t, exists, true)

	exists, err = f.keeper.MinaToCosmosHas(f.ctx, MinaPubKey)
	require.NoError(t, err)
	require.Equal(t, exists, true)

}

// TestInvalidCreatorAddress verifies that RegisterKeys fails with ErrInvalidCreatorAddres
// when the creator field is not a valid bech32 address.
func TestInvalidCreatorAddress(t *testing.T) {

	priv := secp256k1.GenPrivKey()

	pub := priv.PubKey()

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         "creator",
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: pub.Bytes(),
		MinaPublicKey:   MinaPubKey,
	})
	require.ErrorIs(t, err, types.ErrInvalidCreatorAddres)
}

// TestInvalidSigner verifies that RegisterKeys fails with ErrInvalidSigner
// when the creator address does not match the address derived from the provided cosmos public key.
func TestInvalidSigner(t *testing.T) {

	priv := secp256k1.GenPrivKey()

	pub := priv.PubKey()

	secondaryPriv := secp256k1.GenPrivKey()

	secondaryPublic := secondaryPriv.PubKey()

	addr := sdk.AccAddress(secondaryPublic.Address())

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: pub.Bytes(),
		MinaPublicKey:   MinaPubKey,
	})

	require.ErrorIs(t, err, types.ErrInvalidSigner)

}

// TODO: Update require.NoError to require.ErrorIs once the VerifyCosmosSig and VerifyMinaSig is implemented
// TestInvalidSignature currently expects no error since signature verification is not yet implemented.
func TestInvalidSignature(t *testing.T) {

	priv := secp256k1.GenPrivKey()

	pub := priv.PubKey()

	addr := sdk.AccAddress(pub.Address())

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	invalidSig := "cosmosSig"

	_, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: invalidSig,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: pub.Bytes(),
		MinaPublicKey:   MinaPubKey,
	})

	require.NoError(t, err)

}

// TestInsertSecondaryKeysFail verifies that registering the same key pair twice
// fails with ErrSecondaryKeyExists on the second attempt.
func TestInsertSecondaryKeysFail(t *testing.T) {
	f := initFixture(t)

	priv := secp256k1.GenPrivKey()

	pub := priv.PubKey()

	addr := sdk.AccAddress(pub.Address())

	ms := keeper.NewMsgServerImpl(f.keeper)

	// First registration should succeed.
	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: pub.Bytes(),
		MinaPublicKey:   MinaPubKey,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Second registration with the same keys should fail.
	resp, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: pub.Bytes(),
		MinaPublicKey:   MinaPubKey,
	})

	require.ErrorIs(t, err, types.ErrSecondaryKeyExists)
}
