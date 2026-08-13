package types_test

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

type signingVector struct {
	Name                       string `json:"name"`
	ChainID                    string `json:"chain_id"`
	Operation                  string `json:"operation"`
	ActorType                  string `json:"actor_type"`
	CosmosPublicKeyHex         string `json:"cosmos_public_key_hex"`
	CurrentMinaPublicKeyBase64 string `json:"current_mina_public_key_base64"`
	NewMinaPublicKeyBase64     string `json:"new_mina_public_key_base64"`
	NewKeyVersion              uint64 `json:"new_key_version"`
	ChallengeDecimal           string `json:"challenge_decimal"`
	ChallengeBase64            string `json:"challenge_base64"`
}

func TestKeySigningChallengeGoldenVectors(t *testing.T) {
	encoded, err := os.ReadFile("testdata/key_signing_vectors.json")
	require.NoError(t, err)

	var vectors []signingVector
	require.NoError(t, json.Unmarshal(encoded, &vectors))
	require.NotEmpty(t, vectors)

	for _, vector := range vectors {
		t.Run(vector.Name, func(t *testing.T) {
			operation, ok := types.KeySigningOperation_value[vector.Operation]
			require.True(t, ok)
			actorType, ok := types.ActorType_value[vector.ActorType]
			require.True(t, ok)

			cosmosPublicKey, err := hex.DecodeString(vector.CosmosPublicKeyHex)
			require.NoError(t, err)
			currentMinaPublicKey, err := base64.StdEncoding.DecodeString(vector.CurrentMinaPublicKeyBase64)
			require.NoError(t, err)
			newMinaPublicKey, err := base64.StdEncoding.DecodeString(vector.NewMinaPublicKeyBase64)
			require.NoError(t, err)

			challenge, err := types.BuildKeySigningChallenge(types.KeySigningChallengeInput{
				ChainID:              vector.ChainID,
				Operation:            types.KeySigningOperation(operation),
				ActorType:            types.ActorType(actorType),
				CosmosPublicKey:      cosmosPublicKey,
				CurrentMinaPublicKey: currentMinaPublicKey,
				NewMinaPublicKey:     newMinaPublicKey,
				NewKeyVersion:        vector.NewKeyVersion,
			})
			require.NoError(t, err)
			require.Equal(t, vector.ChallengeDecimal, challenge.String())
			require.Equal(t, vector.ChallengeBase64, base64.StdEncoding.EncodeToString(challenge.Bytes()))
		})
	}
}

func TestKeySigningChallengeBindsStateTransition(t *testing.T) {
	base := signingInput(t)
	baseChallenge := challengeBytes(t, base)

	mutations := map[string]func(*types.KeySigningChallengeInput){
		"chain ID": func(input *types.KeySigningChallengeInput) { input.ChainID = "pulsar-other-1" },
		"stable key": func(input *types.KeySigningChallengeInput) {
			input.CosmosPublicKey = secp256k1.GenPrivKey().PubKey().Bytes()
		},
		"current mina key": func(input *types.KeySigningChallengeInput) { input.CurrentMinaPublicKey = testMinaPublicKey(t, 3) },
		"new mina key":     func(input *types.KeySigningChallengeInput) { input.NewMinaPublicKey = testMinaPublicKey(t, 4) },
		"version":          func(input *types.KeySigningChallengeInput) { input.NewKeyVersion++ },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			input := cloneSigningInput(base)
			mutate(&input)
			require.NotEqual(t, baseChallenge, challengeBytes(t, input))
		})
	}

	registration := cloneSigningInput(base)
	registration.Operation = types.KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER
	registration.CurrentMinaPublicKey = nil
	registration.NewKeyVersion = 0
	require.NotEqual(t, baseChallenge, challengeBytes(t, registration))

	validator := cloneSigningInput(base)
	validator.ActorType = types.ActorType_ACTOR_TYPE_VALIDATOR
	validator.CosmosPublicKey = make([]byte, 32)
	validator.CosmosPublicKey[0] = 1
	require.NotEqual(t, baseChallenge, challengeBytes(t, validator))
}

func TestKeySigningChallengeIsDeterministic(t *testing.T) {
	base := signingInput(t)
	require.Equal(t, challengeBytes(t, base), challengeBytes(t, cloneSigningInput(base)))
}

func TestKeySigningChallengeRejectsInvalidTransitions(t *testing.T) {
	base := signingInput(t)
	for _, tc := range []struct {
		name   string
		mutate func(*types.KeySigningChallengeInput)
		errIs  error
	}{
		{name: "empty chain", mutate: func(input *types.KeySigningChallengeInput) { input.ChainID = "" }, errIs: types.ErrInvalidSignature},
		{name: "invalid operation", mutate: func(input *types.KeySigningChallengeInput) {
			input.Operation = types.KeySigningOperation_KEY_SIGNING_OPERATION_UNSPECIFIED
		}, errIs: types.ErrInvalidSigningOperation},
		{name: "invalid actor", mutate: func(input *types.KeySigningChallengeInput) { input.ActorType = types.ActorType_ACTOR_TYPE_UNSPECIFIED }, errIs: types.ErrInvalidActorType},
		{name: "zero update version", mutate: func(input *types.KeySigningChallengeInput) { input.NewKeyVersion = 0 }, errIs: types.ErrInvalidKeyVersion},
		{name: "unchanged key", mutate: func(input *types.KeySigningChallengeInput) {
			input.NewMinaPublicKey = append([]byte(nil), input.CurrentMinaPublicKey...)
		}, errIs: types.ErrUnchangedMinaPublicKey},
		{name: "registration with current state", mutate: func(input *types.KeySigningChallengeInput) {
			input.Operation = types.KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER
		}, errIs: types.ErrInvalidKeyVersion},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := cloneSigningInput(base)
			tc.mutate(&input)
			challenge, err := types.BuildKeySigningChallenge(input)
			require.ErrorIs(t, err, tc.errIs)
			require.Nil(t, challenge)
		})
	}
}

func signingInput(t *testing.T) types.KeySigningChallengeInput {
	t.Helper()
	cosmosKey, err := hex.DecodeString("028e23b60777010732ad6bc2607f5ee5624fbba62ad284bc1300852cf90b2d94b0")
	require.NoError(t, err)
	current := testMinaPublicKey(t, 1)
	newKey := testMinaPublicKey(t, 2)
	return types.KeySigningChallengeInput{
		ChainID: "pulsar-test-1", Operation: types.KeySigningOperation_KEY_SIGNING_OPERATION_UPDATE,
		ActorType: types.ActorType_ACTOR_TYPE_USER, CosmosPublicKey: cosmosKey,
		CurrentMinaPublicKey: current, NewMinaPublicKey: newKey, NewKeyVersion: 1,
	}
}

func testMinaPublicKey(t *testing.T, scalar byte) []byte {
	t.Helper()
	var seed [32]byte
	seed[0] = scalar
	privateKey, err := privatekey.NewPrivateKeyFromBytes(seed, mina.TestNet)
	require.NoError(t, err)
	publicKey, err := privateKey.ToPublicKey()
	require.NoError(t, err)
	return publicKey.Bytes()
}

func cloneSigningInput(input types.KeySigningChallengeInput) types.KeySigningChallengeInput {
	input.CosmosPublicKey = append([]byte(nil), input.CosmosPublicKey...)
	input.CurrentMinaPublicKey = append([]byte(nil), input.CurrentMinaPublicKey...)
	input.NewMinaPublicKey = append([]byte(nil), input.NewMinaPublicKey...)
	return input
}

func challengeBytes(t *testing.T, input types.KeySigningChallengeInput) []byte {
	t.Helper()
	challenge, err := types.BuildKeySigningChallenge(input)
	require.NoError(t, err)
	return challenge.Bytes()
}
