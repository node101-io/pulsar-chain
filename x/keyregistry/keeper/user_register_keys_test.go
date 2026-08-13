package keeper_test

import (
	"bytes"
	"context"
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

func malformedMinaPublicKey() []byte { return bytes.Repeat([]byte{0xff}, publickey.Size()) }
func malformedMinaSignature() []byte { return []byte("bad-mina-signature") }

func generateMinaKey(_ types.ActorType) (*privatekey.PrivateKey, error) {
	return privatekey.NewPrivateKeyFromBytes([32]byte(MinaPriv), mina.TestNet)
}

func generateMinaSecondaryKeyPair(_ types.ActorType) (*privatekey.PrivateKey, error) {
	return privatekey.NewPrivateKeyFromBytes([32]byte(MinaSecondaryPriv), mina.TestNet)
}

func generateUserCosmosPrivKey() secp256k1.PrivKey         { return secp256k1.GenPrivKey() }
func generateValidatorCosmosPrivKey() cometed25519.PrivKey { return cometed25519.GenPrivKey() }

func minaPublicKey(t *testing.T, privateKey *privatekey.PrivateKey) []byte {
	t.Helper()
	publicKey, err := privateKey.ToPublicKey()
	require.NoError(t, err)
	return publicKey.Bytes()
}

func signChallenge(t *testing.T, privateKey *privatekey.PrivateKey, input types.KeySigningChallengeInput) []byte {
	t.Helper()
	challenge, err := types.BuildKeySigningChallenge(input)
	require.NoError(t, err)
	sig, err := privateKey.SignFieldElement(challenge)
	require.NoError(t, err)
	return sig.Bytes()
}

func sdkChainID(ctx context.Context) string { return sdk.UnwrapSDKContext(ctx).ChainID() }

func newUserRegistration(t *testing.T, ctx context.Context, cosmosPrivateKey secp256k1.PrivKey, minaPrivateKey *privatekey.PrivateKey) *types.MsgRegisterUserKeys {
	t.Helper()
	cosmosPublicKey := cosmosPrivateKey.PubKey().Bytes()
	minaKey := minaPublicKey(t, minaPrivateKey)
	return &types.MsgRegisterUserKeys{
		Creator:         sdk.AccAddress(cosmosPrivateKey.PubKey().Address()).String(),
		CosmosPublicKey: cosmosPublicKey,
		MinaPublicKey:   minaKey,
		MinaSignature: signChallenge(t, minaPrivateKey, types.KeySigningChallengeInput{
			ChainID: sdkChainID(ctx), Operation: types.KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER,
			ActorType: types.ActorType_USER, CosmosPublicKey: cosmosPublicKey, NewMinaPublicKey: minaKey,
		}),
	}
}

func TestRegisterUserKeys(t *testing.T) {
	f := initFixture(t)
	server := keeper.NewMsgServerImpl(f.keeper)
	minaPrivateKey, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)
	msg := newUserRegistration(t, f.ctx, generateUserCosmosPrivKey(), minaPrivateKey)

	response, err := server.RegisterUserKeys(f.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, msg.MinaPublicKey, mustUserMinaKey(t, f, msg.CosmosPublicKey))
	version, err := f.keeper.UserGetKeyVersion(f.ctx, msg.CosmosPublicKey)
	require.NoError(t, err)
	require.Zero(t, version)
}

func TestRegisterUserKeysRejectsInvalidProofsWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*types.MsgRegisterUserKeys)
		errIs  error
	}{
		{name: "invalid creator", mutate: func(msg *types.MsgRegisterUserKeys) { msg.Creator = "invalid" }, errIs: types.ErrInvalidCreatorAddress},
		{name: "wrong creator", mutate: func(msg *types.MsgRegisterUserKeys) {
			msg.Creator = sdk.AccAddress(generateUserCosmosPrivKey().PubKey().Address()).String()
		}, errIs: types.ErrInvalidCreatorAddress},
		{name: "malformed mina key", mutate: func(msg *types.MsgRegisterUserKeys) { msg.MinaPublicKey = malformedMinaPublicKey() }, errIs: types.ErrInvalidPublicKey},
		{name: "malformed mina signature", mutate: func(msg *types.MsgRegisterUserKeys) { msg.MinaSignature = malformedMinaSignature() }, errIs: types.ErrInvalidSignature},
		{name: "wrong mina signature", mutate: func(msg *types.MsgRegisterUserKeys) {
			other, err := generateMinaSecondaryKeyPair(types.ActorType_USER)
			require.NoError(t, err)
			msg.MinaSignature = newUserRegistration(t, initFixture(t).ctx, generateUserCosmosPrivKey(), other).MinaSignature
		}, errIs: types.ErrInvalidSignature},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			server := keeper.NewMsgServerImpl(f.keeper)
			minaPrivateKey, err := generateMinaKey(types.ActorType_USER)
			require.NoError(t, err)
			msg := newUserRegistration(t, f.ctx, generateUserCosmosPrivKey(), minaPrivateKey)
			tc.mutate(msg)
			_, err = server.RegisterUserKeys(f.ctx, msg)
			require.ErrorIs(t, err, tc.errIs)
			exists, stateErr := f.keeper.UserCosmosToMinaHas(f.ctx, msg.CosmosPublicKey)
			require.NoError(t, stateErr)
			require.False(t, exists)
		})
	}
}

func TestRegisterUserKeysRejectsDuplicateKeys(t *testing.T) {
	f := initFixture(t)
	server := keeper.NewMsgServerImpl(f.keeper)
	minaPrivateKey, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)
	first := newUserRegistration(t, f.ctx, generateUserCosmosPrivKey(), minaPrivateKey)
	_, err = server.RegisterUserKeys(f.ctx, first)
	require.NoError(t, err)

	_, err = server.RegisterUserKeys(f.ctx, first)
	require.ErrorIs(t, err, types.ErrUserSecondaryKeyExists)
	second := newUserRegistration(t, f.ctx, generateUserCosmosPrivKey(), minaPrivateKey)
	_, err = server.RegisterUserKeys(f.ctx, second)
	require.ErrorIs(t, err, types.ErrUserSecondaryKeyExists)
}

func mustUserMinaKey(t *testing.T, f *fixture, cosmosPublicKey []byte) []byte {
	t.Helper()
	key, err := f.keeper.UserGetCosmosToMina(f.ctx, cosmosPublicKey)
	require.NoError(t, err)
	return key
}
