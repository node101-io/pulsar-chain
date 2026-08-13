package keeper_test

import (
	"testing"

	cometed25519 "github.com/cometbft/cometbft/crypto/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

func newValidatorRegistration(t *testing.T, f *fixture, consensusPrivateKey cometed25519.PrivKey, minaPrivateKey *privatekey.PrivateKey) *types.MsgRegisterValidatorKeys {
	t.Helper()
	consensusPublicKey := consensusPrivateKey.PubKey().Bytes()
	minaKey := minaPublicKey(t, minaPrivateKey)
	challenge, err := types.BuildKeySigningChallenge(types.KeySigningChallengeInput{
		ChainID: sdkChainID(f.ctx), Operation: types.KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER,
		ActorType: types.ActorType_ACTOR_TYPE_VALIDATOR, CosmosPublicKey: consensusPublicKey, NewMinaPublicKey: minaKey,
	})
	require.NoError(t, err)
	minaSignature, err := minaPrivateKey.SignFieldElement(challenge)
	require.NoError(t, err)
	consensusSignature, err := consensusPrivateKey.Sign(challenge.Bytes())
	require.NoError(t, err)
	return &types.MsgRegisterValidatorKeys{
		Creator:                     sdk.AccAddress(generateUserCosmosPrivKey().PubKey().Address()).String(),
		ValidatorConsensusPublicKey: consensusPublicKey, MinaPublicKey: minaKey,
		MinaSignature: minaSignature.Bytes(), ValidatorConsensusSignature: consensusSignature,
	}
}

func TestRegisterValidatorKeysAllowsRelayer(t *testing.T) {
	f := initFixture(t)
	server := keeper.NewMsgServerImpl(f.keeper)
	minaPrivateKey, err := generateMinaKey(types.ActorType_ACTOR_TYPE_VALIDATOR)
	require.NoError(t, err)
	msg := newValidatorRegistration(t, f, generateValidatorCosmosPrivKey(), minaPrivateKey)

	response, err := server.RegisterValidatorKeys(f.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := f.keeper.ValidatorGetCosmosToMina(f.ctx, msg.ValidatorConsensusPublicKey)
	require.NoError(t, err)
	require.Equal(t, msg.MinaPublicKey, stored)
}

func TestRegisterValidatorKeysRejectsEitherInvalidProof(t *testing.T) {
	for _, field := range []string{"mina", "consensus"} {
		t.Run(field, func(t *testing.T) {
			f := initFixture(t)
			server := keeper.NewMsgServerImpl(f.keeper)
			minaPrivateKey, err := generateMinaKey(types.ActorType_ACTOR_TYPE_VALIDATOR)
			require.NoError(t, err)
			msg := newValidatorRegistration(t, f, generateValidatorCosmosPrivKey(), minaPrivateKey)
			if field == "mina" {
				msg.MinaSignature = malformedMinaSignature()
			} else {
				msg.ValidatorConsensusSignature = []byte("wrong")
			}
			_, err = server.RegisterValidatorKeys(f.ctx, msg)
			require.ErrorIs(t, err, types.ErrInvalidSignature)
			exists, stateErr := f.keeper.ValidatorCosmosToMinaHas(f.ctx, msg.ValidatorConsensusPublicKey)
			require.NoError(t, stateErr)
			require.False(t, exists)
		})
	}
}
