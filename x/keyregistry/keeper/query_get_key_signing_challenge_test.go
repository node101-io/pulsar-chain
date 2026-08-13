package keeper_test

import (
	"testing"

	minafield "github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetKeySigningChallengeRegistrationAndUpdate(t *testing.T) {
	f := initFixture(t)
	queryServer := keeper.NewQueryServerImpl(f.keeper)
	msgServer := keeper.NewMsgServerImpl(f.keeper)
	cosmosPrivateKey := generateUserCosmosPrivKey()
	minaPrivateKey, err := generateMinaKey(types.ActorType_ACTOR_TYPE_USER)
	require.NoError(t, err)
	registration := newUserRegistration(t, f.ctx, cosmosPrivateKey, minaPrivateKey)

	registerResponse, err := queryServer.GetKeySigningChallenge(f.ctx, &types.QueryGetKeySigningChallengeRequest{
		Operation: types.KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER, ActorType: types.ActorType_ACTOR_TYPE_USER,
		CosmosPublicKey: registration.CosmosPublicKey, NewMinaPublicKey: registration.MinaPublicKey,
	})
	require.NoError(t, err)
	require.Empty(t, registerResponse.PreviousMinaPublicKey)
	require.Zero(t, registerResponse.NewKeyVersion)
	assertChallengeResponse(t, registerResponse)

	_, err = msgServer.RegisterUserKeys(f.ctx, registration)
	require.NoError(t, err)
	nextPrivateKey, err := generateMinaSecondaryKeyPair(types.ActorType_ACTOR_TYPE_USER)
	require.NoError(t, err)
	updateResponse, err := queryServer.GetKeySigningChallenge(f.ctx, &types.QueryGetKeySigningChallengeRequest{
		Operation: types.KeySigningOperation_KEY_SIGNING_OPERATION_UPDATE, ActorType: types.ActorType_ACTOR_TYPE_USER,
		CosmosPublicKey: registration.CosmosPublicKey, NewMinaPublicKey: minaPublicKey(t, nextPrivateKey),
	})
	require.NoError(t, err)
	require.Equal(t, registration.MinaPublicKey, updateResponse.PreviousMinaPublicKey)
	require.EqualValues(t, 1, updateResponse.NewKeyVersion)
	assertChallengeResponse(t, updateResponse)
}

func TestGetKeySigningChallengeRejectsInvalidState(t *testing.T) {
	f := initFixture(t)
	queryServer := keeper.NewQueryServerImpl(f.keeper)
	cosmosPrivateKey := generateUserCosmosPrivKey()
	minaPrivateKey, err := generateMinaKey(types.ActorType_ACTOR_TYPE_USER)
	require.NoError(t, err)
	registration := newUserRegistration(t, f.ctx, cosmosPrivateKey, minaPrivateKey)

	_, err = queryServer.GetKeySigningChallenge(f.ctx, &types.QueryGetKeySigningChallengeRequest{
		Operation: types.KeySigningOperation_KEY_SIGNING_OPERATION_UPDATE, ActorType: types.ActorType_ACTOR_TYPE_USER,
		CosmosPublicKey: registration.CosmosPublicKey, NewMinaPublicKey: registration.MinaPublicKey,
	})
	require.Equal(t, codes.NotFound, status.Code(err))

	_, err = keeper.NewMsgServerImpl(f.keeper).RegisterUserKeys(f.ctx, registration)
	require.NoError(t, err)
	_, err = queryServer.GetKeySigningChallenge(f.ctx, &types.QueryGetKeySigningChallengeRequest{
		Operation: types.KeySigningOperation_KEY_SIGNING_OPERATION_UPDATE, ActorType: types.ActorType_ACTOR_TYPE_USER,
		CosmosPublicKey: registration.CosmosPublicKey, NewMinaPublicKey: registration.MinaPublicKey,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = queryServer.GetKeySigningChallenge(f.ctx, &types.QueryGetKeySigningChallengeRequest{
		Operation: types.KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER, ActorType: types.ActorType_ACTOR_TYPE_USER,
		CosmosPublicKey: registration.CosmosPublicKey, NewMinaPublicKey: registration.MinaPublicKey,
	})
	require.Equal(t, codes.AlreadyExists, status.Code(err))
}

func assertChallengeResponse(t *testing.T, response *types.QueryGetKeySigningChallengeResponse) {
	t.Helper()
	challenge, err := minafield.NewFieldElement(response.ChallengeBytes)
	require.NoError(t, err)
	require.Equal(t, response.Challenge, challenge.String())
}
