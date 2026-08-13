package keeper_test

import (
	"testing"

	cometed25519 "github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

func registerValidatorForUpdate(t *testing.T, f *fixture) (cometed25519.PrivKey, []byte) {
	t.Helper()
	consensusPrivateKey := generateValidatorCosmosPrivKey()
	minaPrivateKey, err := generateMinaKey(types.ActorType_ACTOR_TYPE_VALIDATOR)
	require.NoError(t, err)
	msg := newValidatorRegistration(t, f, consensusPrivateKey, minaPrivateKey)
	_, err = keeper.NewMsgServerImpl(f.keeper).RegisterValidatorKeys(f.ctx, msg)
	require.NoError(t, err)
	return consensusPrivateKey, msg.MinaPublicKey
}

func newValidatorUpdate(t *testing.T, f *fixture, consensusPrivateKey cometed25519.PrivKey, currentMinaPublicKey []byte, nextMinaPrivateKey *privatekey.PrivateKey, version uint64) *types.MsgUpdateValidatorKeys {
	t.Helper()
	consensusPublicKey := consensusPrivateKey.PubKey().Bytes()
	newMinaPublicKey := minaPublicKey(t, nextMinaPrivateKey)
	challenge, err := types.BuildKeySigningChallenge(types.KeySigningChallengeInput{
		ChainID: sdkChainID(f.ctx), Operation: types.KeySigningOperation_KEY_SIGNING_OPERATION_UPDATE,
		ActorType: types.ActorType_ACTOR_TYPE_VALIDATOR, CosmosPublicKey: consensusPublicKey,
		CurrentMinaPublicKey: currentMinaPublicKey, NewMinaPublicKey: newMinaPublicKey, NewKeyVersion: version,
	})
	require.NoError(t, err)
	minaSignature, err := nextMinaPrivateKey.SignFieldElement(challenge)
	require.NoError(t, err)
	consensusSignature, err := consensusPrivateKey.Sign(challenge.Bytes())
	require.NoError(t, err)
	return &types.MsgUpdateValidatorKeys{
		Creator:                     newValidatorRegistration(t, f, consensusPrivateKey, nextMinaPrivateKey).Creator,
		ValidatorConsensusPublicKey: consensusPublicKey, NewMinaPublicKey: newMinaPublicKey,
		NewKeyVersion: version, NewMinaSignature: minaSignature.Bytes(), ValidatorConsensusSignature: consensusSignature,
	}
}

func TestUpdateValidatorKeysAndRejectReplay(t *testing.T) {
	f := initFixture(t)
	server := keeper.NewMsgServerImpl(f.keeper)
	consensusPrivateKey, keyA := registerValidatorForUpdate(t, f)
	keyBPrivate, err := generateMinaSecondaryKeyPair(types.ActorType_ACTOR_TYPE_VALIDATOR)
	require.NoError(t, err)
	updateAB := newValidatorUpdate(t, f, consensusPrivateKey, keyA, keyBPrivate, 1)
	_, err = server.UpdateValidatorKeys(f.ctx, updateAB)
	require.NoError(t, err)

	keyAPrivate, err := generateMinaKey(types.ActorType_ACTOR_TYPE_VALIDATOR)
	require.NoError(t, err)
	staleBA := newValidatorUpdate(t, f, consensusPrivateKey, updateAB.NewMinaPublicKey, keyAPrivate, 1)
	_, err = server.UpdateValidatorKeys(f.ctx, staleBA)
	require.ErrorIs(t, err, types.ErrStaleKeyVersion)

	freshBA := newValidatorUpdate(t, f, consensusPrivateKey, updateAB.NewMinaPublicKey, keyAPrivate, 2)
	_, err = server.UpdateValidatorKeys(f.ctx, freshBA)
	require.NoError(t, err)
	stored, err := f.keeper.ValidatorGetCosmosToMina(f.ctx, updateAB.ValidatorConsensusPublicKey)
	require.NoError(t, err)
	require.Equal(t, keyA, stored)
}

func TestUpdateValidatorKeysRequiresBothProofsWithoutMutation(t *testing.T) {
	for _, proof := range []string{"mina", "consensus"} {
		t.Run(proof, func(t *testing.T) {
			f := initFixture(t)
			server := keeper.NewMsgServerImpl(f.keeper)
			consensusPrivateKey, current := registerValidatorForUpdate(t, f)
			nextPrivateKey, err := generateMinaSecondaryKeyPair(types.ActorType_ACTOR_TYPE_VALIDATOR)
			require.NoError(t, err)
			msg := newValidatorUpdate(t, f, consensusPrivateKey, current, nextPrivateKey, 1)
			if proof == "mina" {
				msg.NewMinaSignature = malformedMinaSignature()
			} else {
				msg.ValidatorConsensusSignature = []byte("wrong")
			}
			_, err = server.UpdateValidatorKeys(f.ctx, msg)
			require.ErrorIs(t, err, types.ErrInvalidSignature)
			stored, stateErr := f.keeper.ValidatorGetCosmosToMina(f.ctx, msg.ValidatorConsensusPublicKey)
			require.NoError(t, stateErr)
			require.Equal(t, current, stored)
			version, stateErr := f.keeper.ValidatorGetKeyVersion(f.ctx, msg.ValidatorConsensusPublicKey)
			require.NoError(t, stateErr)
			require.Zero(t, version)
		})
	}
}
