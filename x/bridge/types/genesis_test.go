package types_test

import (
	"testing"

	"github.com/node101-io/pulsar-chain/x/bridge/types"
	"github.com/stretchr/testify/require"
)

const (
	testConfirmationDepth int64 = 32
	testContractAddress         = "B62qjRDirGFRf5dvNcGzMs5oWzQ2VyNcygnoKM2MkxB9PFUp7Utdraf"
)

func validBridgeParams() types.Params {
	return types.NewParams(testConfirmationDepth, testContractAddress)
}

func TestGenesisState_Validate(t *testing.T) {
	tests := []struct {
		desc     string
		genState *types.GenesisState
		valid    bool
	}{
		{
			desc:     "default is invalid",
			genState: types.DefaultGenesis(),
			valid:    false,
		},
		{
			desc:     "empty genesis state is invalid",
			genState: &types.GenesisState{},
			valid:    false,
		},
		{
			desc: "explicit valid genesis state",
			genState: &types.GenesisState{
				Params:                      validBridgeParams(),
				BridgeState:                 types.DefaultBridgeState(),
				ActionsReducedRootSnapshots: types.DefaultActionsReducedRootSnapshots(),
			},
			valid: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}
