package keeper_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

func generateValidatorPublicKeys() (cryptotypes.PubKey, []byte, error) {
	cosmosPrivKey := ed25519.GenPrivKey()

	cosmosPubKey := cosmosPrivKey.PubKey()

	minaPrivKey := keys.NewPrivateKeyFromBytes([32]byte(MinaPriv))

	minaAddress, err := minaPrivKey.ToPublicKey().Marshal()

	return cosmosPubKey, minaAddress, err
}

// TestValidatorRegisterKeysFail verifies that RegisterKeys fails with ErrInvalidPublicKey
// when the provided cosmos consensus public key is invalid.
func TestValidatorRegisterKeysFail(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	creatorAddr := sdk.ConsAddress([]byte("pulsar"))
	_, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   CosmosPubKey,
		MinaAddress:     MinaPubKey,
		IsUser:          false,
	})
	require.ErrorIs(t, err, types.ErrInvalidPublicKey)
}

// TestValidatorRegisterKeysSuccess verifies that RegisterKeys succeeds with valid inputs
// and ensures that both CosmosToMina and MinaToCosmos mappings are correctly stored.
func TestValidatorRegisterKeysSuccess(t *testing.T) {

	cosmosPubKey, minaPubKey, err := generateValidatorPublicKeys()
	require.NoError(t, err)

	addr := sdk.ConsAddress(cosmosPubKey.Address())

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   cosmosPubKey.Bytes(),
		MinaAddress:     minaPubKey,
		IsUser:          false,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	exists, err := f.keeper.ValidatorCosmosToMinaHas(f.ctx, cosmosPubKey.Bytes())
	require.NoError(t, err)
	require.Equal(t, exists, true)

	exists, err = f.keeper.ValidatorMinaToCosmosHas(f.ctx, minaPubKey)
	require.NoError(t, err)
	require.Equal(t, exists, true)
}

// TestValidatorInvalidCreatorAddress verifies that RegisterKeys fails with ErrInvalidCreatorAddres
// when the creator field is not a valid bech32 address.
func TestValidatorInvalidCreatorAddress(t *testing.T) {

	cosmosPubKey, minaPubKey, err := generateValidatorPublicKeys()
	require.NoError(t, err)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         "creator",
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   cosmosPubKey.Bytes(),
		MinaAddress:     minaPubKey,
		IsUser:          false,
	})
	require.ErrorIs(t, err, types.ErrInvalidCreatorAddres)
}

// TestValidatorInvalidSigner verifies that RegisterKeys fails with ErrInvalidSigner
// when the creator address does not match the address derived from the provided consensus public key.
func TestValidatorInvalidSigner(t *testing.T) {

	cosmosPubKey, minaPubKey, err := generateValidatorPublicKeys()
	require.NoError(t, err)

	secondaryPriv := ed25519.GenPrivKey()

	secondaryPublic := secondaryPriv.PubKey()

	addr := sdk.ConsAddress(secondaryPublic.Address())

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   cosmosPubKey.Bytes(),
		MinaAddress:     minaPubKey,
		IsUser:          false,
	})

	require.ErrorIs(t, err, types.ErrInvalidSigner)
}

// TODO: Update require.NoError to require.ErrorIs once the VerifyCosmosSig and VerifyMinaSig is implemented
// TestValidatorInvalidSignature currently expects no error since signature verification is not yet implemented.
func TestValidatorInvalidSignature(t *testing.T) {

	cosmosPubKey, minaPubKey, err := generateValidatorPublicKeys()
	addr := sdk.ConsAddress(cosmosPubKey.Address())

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
		IsUser:          false,
	})

	require.NoError(t, err)

}

// TestValidatorInsertSecondaryKeysFail verifies that registering the same key pair twice
// fails with ErrSecondaryKeyExists on the second attempt.
func TestValidatorInsertSecondaryKeysFail(t *testing.T) {
	f := initFixture(t)

	cosmosPubKey, minaPubKey, err := generateValidatorPublicKeys()
	require.NoError(t, err)

	addr := sdk.ConsAddress(cosmosPubKey.Address())

	ms := keeper.NewMsgServerImpl(f.keeper)

	// First registration should succeed.
	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         addr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosAddress:   cosmosPubKey.Bytes(),
		MinaAddress:     minaPubKey,
		IsUser:          false,
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
		IsUser:          false,
	})

	require.ErrorIs(t, err, types.ErrValidatorSecondaryKeyExists)
}
