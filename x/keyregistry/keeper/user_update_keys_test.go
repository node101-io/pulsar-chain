package keeper_test

import (
	"testing"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

func registerUserForUpdate(t *testing.T, f *fixture) (secp256k1.PrivKey, []byte) {
	t.Helper()
	cosmosPrivateKey := generateUserCosmosPrivKey()
	minaPrivateKey, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)
	msg := newUserRegistration(t, f.ctx, cosmosPrivateKey, minaPrivateKey)
	_, err = keeper.NewMsgServerImpl(f.keeper).RegisterUserKeys(f.ctx, msg)
	require.NoError(t, err)
	return cosmosPrivateKey, msg.MinaPublicKey
}

func newUserUpdate(t *testing.T, f *fixture, cosmosPrivateKey secp256k1.PrivKey, currentMinaPublicKey []byte, nextMinaPrivateKey *privatekey.PrivateKey, version uint64, chainID string) *types.MsgUpdateUserKeys {
	t.Helper()
	cosmosPublicKey := cosmosPrivateKey.PubKey().Bytes()
	newMinaPublicKey := minaPublicKey(t, nextMinaPrivateKey)
	return &types.MsgUpdateUserKeys{
		Creator: sdk.AccAddress(cosmosPrivateKey.PubKey().Address()).String(), CosmosPublicKey: cosmosPublicKey,
		NewMinaPublicKey: newMinaPublicKey, NewKeyVersion: version,
		NewMinaSignature: signChallenge(t, nextMinaPrivateKey, types.KeySigningChallengeInput{
			ChainID: chainID, Operation: types.KeySigningOperation_KEY_SIGNING_OPERATION_UPDATE,
			ActorType: types.ActorType_USER, CosmosPublicKey: cosmosPublicKey,
			CurrentMinaPublicKey: currentMinaPublicKey, NewMinaPublicKey: newMinaPublicKey, NewKeyVersion: version,
		}),
	}
}

func TestUpdateUserKeysAndRejectReplay(t *testing.T) {
	f := initFixture(t)
	server := keeper.NewMsgServerImpl(f.keeper)
	cosmosPrivateKey, keyA := registerUserForUpdate(t, f)
	keyBPrivate, err := generateMinaSecondaryKeyPair(types.ActorType_USER)
	require.NoError(t, err)
	updateAB := newUserUpdate(t, f, cosmosPrivateKey, keyA, keyBPrivate, 1, sdkChainID(f.ctx))

	_, err = server.UpdateUserKeys(f.ctx, updateAB)
	require.NoError(t, err)
	keyB := updateAB.NewMinaPublicKey
	require.Equal(t, keyB, mustUserMinaKey(t, f, updateAB.CosmosPublicKey))
	version, err := f.keeper.UserGetKeyVersion(f.ctx, updateAB.CosmosPublicKey)
	require.NoError(t, err)
	require.EqualValues(t, 1, version)

	keyAPrivate, err := generateMinaKey(types.ActorType_USER)
	require.NoError(t, err)
	staleBA := newUserUpdate(t, f, cosmosPrivateKey, keyB, keyAPrivate, 1, sdkChainID(f.ctx))
	_, err = server.UpdateUserKeys(f.ctx, staleBA)
	require.ErrorIs(t, err, types.ErrStaleKeyVersion)
	require.Equal(t, keyB, mustUserMinaKey(t, f, updateAB.CosmosPublicKey))

	freshBA := newUserUpdate(t, f, cosmosPrivateKey, keyB, keyAPrivate, 2, sdkChainID(f.ctx))
	_, err = server.UpdateUserKeys(f.ctx, freshBA)
	require.NoError(t, err)
	require.Equal(t, keyA, mustUserMinaKey(t, f, updateAB.CosmosPublicKey))
}

func TestUpdateUserKeysRejectsWrongContextWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*types.MsgUpdateUserKeys)
		errIs  error
	}{
		{name: "wrong chain signature", mutate: func(msg *types.MsgUpdateUserKeys) {}, errIs: types.ErrInvalidSignature},
		{name: "stale version", mutate: func(msg *types.MsgUpdateUserKeys) { msg.NewKeyVersion = 2 }, errIs: types.ErrStaleKeyVersion},
		{name: "wrong creator", mutate: func(msg *types.MsgUpdateUserKeys) {
			msg.Creator = sdk.AccAddress(generateUserCosmosPrivKey().PubKey().Address()).String()
		}, errIs: types.ErrInvalidCreatorAddress},
		{name: "malformed signature", mutate: func(msg *types.MsgUpdateUserKeys) { msg.NewMinaSignature = malformedMinaSignature() }, errIs: types.ErrInvalidSignature},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := initFixture(t)
			server := keeper.NewMsgServerImpl(f.keeper)
			cosmosPrivateKey, current := registerUserForUpdate(t, f)
			nextPrivateKey, err := generateMinaSecondaryKeyPair(types.ActorType_USER)
			require.NoError(t, err)
			signingChainID := sdkChainID(f.ctx)
			if tc.name == "wrong chain signature" {
				signingChainID = "another-pulsar-chain"
			}
			msg := newUserUpdate(t, f, cosmosPrivateKey, current, nextPrivateKey, 1, signingChainID)
			tc.mutate(msg)
			_, err = server.UpdateUserKeys(f.ctx, msg)
			require.ErrorIs(t, err, tc.errIs)
			require.Equal(t, current, mustUserMinaKey(t, f, msg.CosmosPublicKey))
			version, stateErr := f.keeper.UserGetKeyVersion(f.ctx, msg.CosmosPublicKey)
			require.NoError(t, stateErr)
			require.Zero(t, version)
		})
	}
}

func TestUpdateUserKeysRejectsUnregisteredAndDuplicateKey(t *testing.T) {
	f := initFixture(t)
	server := keeper.NewMsgServerImpl(f.keeper)
	firstCosmosPrivateKey, firstMinaKey := registerUserForUpdate(t, f)
	secondCosmosPrivateKey := generateUserCosmosPrivKey()
	secondMinaPrivateKey, err := generateMinaSecondaryKeyPair(types.ActorType_USER)
	require.NoError(t, err)
	secondRegistration := newUserRegistration(t, f.ctx, secondCosmosPrivateKey, secondMinaPrivateKey)
	_, err = server.RegisterUserKeys(f.ctx, secondRegistration)
	require.NoError(t, err)

	duplicate := newUserUpdate(t, f, firstCosmosPrivateKey, firstMinaKey, secondMinaPrivateKey, 1, sdkChainID(f.ctx))
	_, err = server.UpdateUserKeys(f.ctx, duplicate)
	require.ErrorIs(t, err, types.ErrUserSecondaryKeyExists)

	unregisteredPrivateKey := generateUserCosmosPrivKey()
	unregistered := newUserUpdate(t, f, unregisteredPrivateKey, firstMinaKey, secondMinaPrivateKey, 1, sdkChainID(f.ctx))
	_, err = server.UpdateUserKeys(f.ctx, unregistered)
	require.ErrorIs(t, err, types.ErrUserNotRegistered)
}
