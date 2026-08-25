package types_test

import (
	"bytes"
	"testing"

	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	tests := []struct {
		desc     string
		genState *types.GenesisState
		valid    bool
	}{
		{
			desc:     "default requires verification key hash",
			genState: types.DefaultGenesis(),
			valid:    false,
		},
		{
			desc: "valid genesis state",
			genState: &types.GenesisState{
				Params: types.NewParams(bytes.Repeat([]byte{0x01}, types.VerificationKeyHashSize)),
				SmartAccounts: []types.SmartAccountEntry{
					{
						Identity: bytes.Repeat([]byte{0x02}, types.IdentitySize),
						Account: types.SmartAccount{SessionKeys: []types.SessionKey{
							{
								PublicKey:       bytes.Repeat([]byte{0x03}, types.SessionPublicKeySize),
								ExpiresAtHeight: 100,
							},
						}},
					},
				},
			},
			valid: true,
		},
		{
			desc:     "missing verification key hash",
			genState: &types.GenesisState{},
			valid:    false,
		},
		{
			desc: "invalid smart account identity",
			genState: &types.GenesisState{
				Params: types.NewParams(bytes.Repeat([]byte{0x01}, types.VerificationKeyHashSize)),
				SmartAccounts: []types.SmartAccountEntry{
					{Identity: []byte{0x01}},
				},
			},
			valid: false,
		},
		{
			desc: "duplicate smart account identity",
			genState: &types.GenesisState{
				Params: types.NewParams(bytes.Repeat([]byte{0x01}, types.VerificationKeyHashSize)),
				SmartAccounts: []types.SmartAccountEntry{
					{Identity: bytes.Repeat([]byte{0x02}, types.IdentitySize)},
					{Identity: bytes.Repeat([]byte{0x02}, types.IdentitySize)},
				},
			},
			valid: false,
		},
		{
			desc: "invalid session key",
			genState: &types.GenesisState{
				Params: types.NewParams(bytes.Repeat([]byte{0x01}, types.VerificationKeyHashSize)),
				SmartAccounts: []types.SmartAccountEntry{
					{
						Identity: bytes.Repeat([]byte{0x02}, types.IdentitySize),
						Account: types.SmartAccount{SessionKeys: []types.SessionKey{
							{PublicKey: []byte{0x03}, ExpiresAtHeight: 100},
						}},
					},
				},
			},
			valid: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
