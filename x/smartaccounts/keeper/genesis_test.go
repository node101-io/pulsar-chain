package keeper_test

import (
	"bytes"
	"testing"

	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"

	"github.com/stretchr/testify/require"
)

func TestGenesis(t *testing.T) {
	identity := bytes.Repeat([]byte{0x02}, types.IdentitySize)
	genesisState := types.GenesisState{
		Params: types.NewParams(bytes.Repeat([]byte{0x01}, types.VerificationKeyHashSize)),
		SmartAccounts: []types.SmartAccountEntry{
			{
				Identity: identity,
				Account: types.SmartAccount{SessionKeys: []types.SessionKey{
					{
						PublicKey:       bytes.Repeat([]byte{0x03}, types.SessionPublicKeySize),
						ExpiresAtHeight: 100,
					},
				}},
			},
		},
	}

	f := initFixture(t)
	err := f.keeper.InitGenesis(f.ctx, genesisState)
	require.NoError(t, err)
	got, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.EqualExportedValues(t, genesisState, *got)

	exists, err := f.keeper.HasSmartAccount(f.ctx, identity)
	require.NoError(t, err)
	require.True(t, exists)
}
