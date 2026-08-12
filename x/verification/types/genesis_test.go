package types_test

import (
	"testing"

	"github.com/node101-io/pulsar-chain/x/verification/types"
	"github.com/stretchr/testify/require"
)

func TestDefaultGenesis(t *testing.T) {
	genesisState := types.DefaultGenesis()

	require.Equal(t, int64(6), genesisState.Params.PendingProofBlocksWindowSize)
	require.Equal(t, int64(256), genesisState.Params.MaxProofRange)
}

func TestGenesisState_Validate(t *testing.T) {
	tests := []struct {
		desc     string
		genState *types.GenesisState
		valid    bool
	}{
		{
			desc:     "default is valid",
			genState: types.DefaultGenesis(),
			valid:    true,
		},
		{
			desc: "valid genesis state",
			genState: &types.GenesisState{
				Params: types.NewParams(6, 256),
			},
			valid: true,
		},
		{
			desc: "invalid pending proof window size",
			genState: &types.GenesisState{
				Params: types.NewParams(0, 256),
			},
			valid: false,
		},
		{
			desc: "invalid max proof range",
			genState: &types.GenesisState{
				Params: types.NewParams(6, 0),
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
