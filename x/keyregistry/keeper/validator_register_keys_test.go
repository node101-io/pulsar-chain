package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

// TestValidatorRegisterKeysFail verifies that RegisterKeys fails with ErrInvalidPublicKey
// when the provided cosmos consensus public key is invalid.
func TestValidatorRegisterKeysFail(t *testing.T) {

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPublicKey, _, _, err := generateValidatorPublicKeys()
	require.NoError(t, err)
	require.NotNil(t, cosmosPublicKey)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: CosmosPubKey,
		MinaPublicKey:   MinaPubKey,
		ActorType:       types.ActorType_VALIDATOR,
	})

	require.ErrorIs(t, err, types.ErrInvalidPublicKey)
}

// TestValidatorRegisterKeysSuccess verifies that RegisterKeys succeeds with valid inputs
// and ensures that both CosmosToMina and MinaToCosmos mappings are correctly stored.
func TestValidatorRegisterKeysSuccess(t *testing.T) {

	cosmosPubKey, minaPubKey, _, err := generateValidatorPublicKeys()
	require.NoError(t, err)
	require.NotNil(t, cosmosPubKey)
	require.NotNil(t, minaPubKey)

	creatorAddr := sdk.AccAddress(cosmosPubKey.Address())
	require.NotNil(t, creatorAddr)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPubKey.Bytes(),
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_VALIDATOR,
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

	cosmosPubKey, minaPubKey, _, err := generateValidatorPublicKeys()

	require.NoError(t, err)
	require.NotNil(t, cosmosPubKey)
	require.NotNil(t, minaPubKey)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         "creator",
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPubKey.Bytes(),
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_VALIDATOR,
	})

	require.ErrorIs(t, err, types.ErrInvalidCreatorAddress)
}

// TODO: Update require.NoError to require.ErrorIs once the VerifyCosmosSig and VerifyMinaSig is implemented
// TestValidatorInvalidSignature currently expects no error since signature verification is not yet implemented.
func TestValidatorInvalidSignature(t *testing.T) {

	cosmosPubKey, minaPubKey, _, err := generateValidatorPublicKeys()
	require.NoError(t, err)

	require.NotNil(t, cosmosPubKey)
	require.NotNil(t, minaPubKey)

	creatorAddr := sdk.AccAddress(cosmosPubKey.Address())
	require.NotNil(t, creatorAddr)

	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	invalidSig := []byte("cosmosSig")

	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: invalidSig,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPubKey.Bytes(),
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_VALIDATOR,
	})

	require.NoError(t, err)

}

// TestValidatorInsertSecondaryKeysFail verifies that registering the same key pair twice
// fails with ErrSecondaryKeyExists on the second attempt.
func TestValidatorInsertSecondaryKeysFail(t *testing.T) {
	f := initFixture(t)

	cosmosPubKey, minaPubKey, _, err := generateValidatorPublicKeys()
	require.NoError(t, err)

	require.NotNil(t, cosmosPubKey)
	require.NotNil(t, minaPubKey)

	creatorAddr := sdk.AccAddress(cosmosPubKey.Address())

	ms := keeper.NewMsgServerImpl(f.keeper)

	// First registration should succeed.
	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPubKey.Bytes(),
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_VALIDATOR,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	// Second registration with the same keys should fail.
	_, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPubKey.Bytes(),
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_VALIDATOR,
	})

	require.ErrorIs(t, err, types.ErrValidatorSecondaryKeyExists)
}
