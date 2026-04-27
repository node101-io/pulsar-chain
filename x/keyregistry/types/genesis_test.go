package types_test

import (
	"bytes"
	"testing"

	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	userCosmosA := testBytes(33, 'a')
	userCosmosB := testBytes(33, 'b')
	userMinaX := testBytes(33, 'x')
	userMinaY := testBytes(33, 'y')
	validatorConsensusA := testBytes(32, 'a')
	validatorConsensusB := testBytes(32, 'b')
	validatorMinaX := testBytes(33, 'x')
	validatorMinaY := testBytes(33, 'y')

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
			desc: "invalid validator key length is invalid",
			genState: withDefaultParams(&types.GenesisState{
				ValidatorKeyPairs: []*types.ValidatorPublicKeyPair{
					validatorPair(testBytes(33, 'a'), validatorMinaX),
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
