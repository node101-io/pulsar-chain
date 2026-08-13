package keeper

import (
	"testing"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	cometed25519 "github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

var internalMinaPriv = []byte("7olA5Knafb5E2hJoWFzD+oamtyXIXXUZmYG9+pBMjTGIjqZTVLNGbE7DQ3Zq5YL5NMW31UMMMGgNCeEk+gyzRA==")
var internalMinaSecondaryPriv = []byte("0GUKibsJSZwgiU7k4cXQQWb2QKEP9/iRFATJEUqf2Pc+GxciLMKRQGTIcInKsTzV09rjDsLmZiBl9Up71bvV6g==")

func TestUserUpdateKeysMissingCosmosToMinaMapping(t *testing.T) {
	ctx, k := initInternalGenesisFixture(t)
	ms := NewMsgServerImpl(k)

	cosmosPriv := secp256k1.GenPrivKey()
	cosmosPubKey := cosmosPriv.PubKey().Bytes()
	prevMinaPubKey := internalMinaPublicKey(t, internalMinaPriv)
	newMinaPubKey := internalMinaPublicKey(t, internalMinaSecondaryPriv)

	err := k.userMinaToCosmos.Set(ctx, prevMinaPubKey, cosmosPubKey)
	require.NoError(t, err)

	_, err = ms.UpdateUserKeys(ctx, &types.MsgUpdateUserKeys{
		Creator:          sdk.AccAddress(cosmosPriv.PubKey().Address()).String(),
		CosmosPublicKey:  cosmosPubKey,
		NewMinaPublicKey: newMinaPubKey,
		NewKeyVersion:    1,
	})
	require.ErrorIs(t, err, types.ErrUserNotRegistered)
}

func TestValidatorUpdateKeysMissingCosmosToMinaMapping(t *testing.T) {
	ctx, k := initInternalGenesisFixture(t)
	ms := NewMsgServerImpl(k)

	cosmosPriv := cometed25519.GenPrivKey()
	cosmosPubKey := cosmosPriv.PubKey().Bytes()
	prevMinaPubKey := internalMinaPublicKey(t, internalMinaPriv)
	newMinaPubKey := internalMinaPublicKey(t, internalMinaSecondaryPriv)

	err := k.validatorMinaToCosmos.Set(ctx, prevMinaPubKey, cosmosPubKey)
	require.NoError(t, err)

	_, err = ms.UpdateValidatorKeys(ctx, &types.MsgUpdateValidatorKeys{
		Creator:                     sdk.AccAddress(cosmosPriv.PubKey().Address()).String(),
		ValidatorConsensusPublicKey: cosmosPubKey,
		NewMinaPublicKey:            newMinaPubKey,
		NewKeyVersion:               1,
	})
	require.ErrorIs(t, err, types.ErrValidatorNotRegistered)
}

func internalMinaPublicKey(t *testing.T, data []byte) []byte {
	t.Helper()

	privKey, err := privatekey.NewPrivateKeyFromBytes([32]byte(data), mina.TestNet)
	require.NoError(t, err)

	pubKey, err := privKey.ToPublicKey()
	require.NoError(t, err)

	return pubKey.Bytes()
}
