package types_test

import (
	"bytes"
	"testing"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	cometed25519 "github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

const (
	minaPrivFixtureA = "7olA5Knafb5E2hJoWFzD+oamtyXIXXUZmYG9+pBMjTGIjqZTVLNGbE7DQ3Zq5YL5NMW31UMMMGgNCeEk+gyzRA=="
	minaPrivFixtureB = "0GUKibsJSZwgiU7k4cXQQWb2QKEP9/iRFATJEUqf2Pc+GxciLMKRQGTIcInKsTzV09rjDsLmZiBl9Up71bvV6g=="
)

func TestGenesisState_Validate(t *testing.T) {
	userCosmosA := secp256k1.GenPrivKey().PubKey().Bytes()
	userCosmosB := secp256k1.GenPrivKey().PubKey().Bytes()
	userMinaX := minaPublicKeyBytes(t, minaPrivFixtureA, types.ActorType_USER)
	userMinaY := minaPublicKeyBytes(t, minaPrivFixtureB, types.ActorType_USER)
	malformedMinaKey := testBytes(32, 0xff)

	validatorConsensusA := cometed25519.GenPrivKey().PubKey().Bytes()
	validatorConsensusB := cometed25519.GenPrivKey().PubKey().Bytes()
	validatorMinaX := minaPublicKeyBytes(t, minaPrivFixtureA, types.ActorType_VALIDATOR)
	validatorMinaY := minaPublicKeyBytes(t, minaPrivFixtureB, types.ActorType_VALIDATOR)

	tests := []struct {
		desc        string
		genState    *types.GenesisState
		expectedErr error
	}{
		{
			desc:     "default genesis is valid",
			genState: withDefaultParams(types.DefaultGenesis()),
		},
		{
			desc:     "empty genesis is valid",
			genState: withDefaultParams(&types.GenesisState{}),
		},
		{
			desc: "valid user key pairs are valid",
			genState: withDefaultParams(&types.GenesisState{
				UserKeyPairs: []*types.UserPublicKeyPair{
					userPair(userCosmosA, userMinaX),
				},
			}),
		},
		{
			desc: "valid validator key pairs are valid",
			genState: withDefaultParams(&types.GenesisState{
				ValidatorKeyPairs: []*types.ValidatorPublicKeyPair{
					validatorPair(validatorConsensusA, validatorMinaX),
				},
			}),
		},
		{
			desc: "nil user key pair is invalid",
			genState: withDefaultParams(&types.GenesisState{
				UserKeyPairs: []*types.UserPublicKeyPair{nil},
			}),
			expectedErr: types.ErrNilKeyPair,
		},
		{
			desc: "nil validator key pair is invalid",
			genState: withDefaultParams(&types.GenesisState{
				ValidatorKeyPairs: []*types.ValidatorPublicKeyPair{nil},
			}),
			expectedErr: types.ErrNilKeyPair,
		},
		{
			desc: "invalid user key length is invalid",
			genState: withDefaultParams(&types.GenesisState{
				UserKeyPairs: []*types.UserPublicKeyPair{
					userPair(testBytes(32, 'a'), userMinaX),
				},
			}),
			expectedErr: types.ErrInvalidPublicKey,
		},
		{
			desc: "malformed user mina key is invalid",
			genState: withDefaultParams(&types.GenesisState{
				UserKeyPairs: []*types.UserPublicKeyPair{
					userPair(userCosmosA, malformedMinaKey),
				},
			}),
			expectedErr: types.ErrInvalidPublicKey,
		},
		{
			desc: "invalid validator key length is invalid",
			genState: withDefaultParams(&types.GenesisState{
				ValidatorKeyPairs: []*types.ValidatorPublicKeyPair{
					validatorPair(testBytes(33, 'a'), validatorMinaX),
				},
			}),
			expectedErr: types.ErrInvalidPublicKey,
		},
		{
			desc: "malformed validator mina key is invalid",
			genState: withDefaultParams(&types.GenesisState{
				ValidatorKeyPairs: []*types.ValidatorPublicKeyPair{
					validatorPair(validatorConsensusA, malformedMinaKey),
				},
			}),
			expectedErr: types.ErrInvalidPublicKey,
		},
		{
			desc: "duplicate user cosmos key is invalid",
			genState: withDefaultParams(&types.GenesisState{
				UserKeyPairs: []*types.UserPublicKeyPair{
					userPair(userCosmosA, userMinaX),
					userPair(userCosmosA, userMinaY),
				},
			}),
			expectedErr: types.ErrInvalidGenesisState,
		},
		{
			desc: "duplicate user mina key is invalid",
			genState: withDefaultParams(&types.GenesisState{
				UserKeyPairs: []*types.UserPublicKeyPair{
					userPair(userCosmosA, userMinaX),
					userPair(userCosmosB, userMinaX),
				},
			}),
			expectedErr: types.ErrInvalidGenesisState,
		},
		{
			desc: "duplicate validator consensus key is invalid",
			genState: withDefaultParams(&types.GenesisState{
				ValidatorKeyPairs: []*types.ValidatorPublicKeyPair{
					validatorPair(validatorConsensusA, validatorMinaX),
					validatorPair(validatorConsensusA, validatorMinaY),
				},
			}),
			expectedErr: types.ErrInvalidGenesisState,
		},
		{
			desc: "duplicate validator mina key is invalid",
			genState: withDefaultParams(&types.GenesisState{
				ValidatorKeyPairs: []*types.ValidatorPublicKeyPair{
					validatorPair(validatorConsensusA, validatorMinaX),
					validatorPair(validatorConsensusB, validatorMinaX),
				},
			}),
			expectedErr: types.ErrInvalidGenesisState,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.expectedErr)
			}
		})
	}
}

func minaPublicKeyBytes(t *testing.T, seed string, actorType types.ActorType) []byte {
	t.Helper()

	var privateKeyBytes [32]byte
	copy(privateKeyBytes[:], []byte(seed))

	privKey, err := privatekey.NewPrivateKeyFromBytes(privateKeyBytes, mina.NetworkID(actorType.String()))
	require.NoError(t, err)

	pubKey, err := privKey.ToPublicKey()
	require.NoError(t, err)

	return pubKey.Bytes()
}

func testBytes(length int, value byte) []byte {
	return bytes.Repeat([]byte{value}, length)
}

func withDefaultParams(genState *types.GenesisState) *types.GenesisState {
	genState.Params = types.DefaultParams()
	return genState
}

func userPair(cosmosKey, minaKey []byte) *types.UserPublicKeyPair {
	return &types.UserPublicKeyPair{
		CosmosKey: cosmosKey,
		MinaKey:   minaKey,
	}
}

func validatorPair(consensusKey, minaKey []byte) *types.ValidatorPublicKeyPair {
	return &types.ValidatorPublicKeyPair{
		CosmosKey: consensusKey,
		MinaKey:   minaKey,
	}
}
