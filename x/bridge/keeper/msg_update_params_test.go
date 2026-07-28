package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/bridge/keeper"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

func TestMsgUpdateParams(t *testing.T) {
	f := initFixture(t)
	ms := keeper.NewMsgServerImpl(f.keeper)

	params := validBridgeParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	authorityStr, err := f.addressCodec.BytesToString(f.keeper.GetAuthority())
	require.NoError(t, err)

	updatedParams := types.NewParams(64, testContractAddress)

	testCases := []struct {
		name      string
		input     *types.MsgUpdateParams
		expErr    bool
		expErrMsg string
	}{
		{
			name: "invalid authority",
			input: &types.MsgUpdateParams{
				Authority: "invalid",
				Params:    updatedParams,
			},
			expErr:    true,
			expErrMsg: "invalid authority",
		},
		{
			name: "invalid params",
			input: &types.MsgUpdateParams{
				Authority: authorityStr,
				Params:    types.Params{},
			},
			expErr:    true,
			expErrMsg: "confirmation_depth must be greater than 0",
		},
		{
			name: "invalid contract address",
			input: &types.MsgUpdateParams{
				Authority: authorityStr,
				Params: types.NewParams(
					testConfirmationDepth,
					"not-a-mina-address",
				),
			},
			expErr:    true,
			expErrMsg: "invalid contract_address",
		},
		{
			name: "all good",
			input: &types.MsgUpdateParams{
				Authority: authorityStr,
				Params:    updatedParams,
			},
			expErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ms.UpdateParams(f.ctx, tc.input)

			if tc.expErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expErrMsg)
				return
			}

			require.NoError(t, err)

			got, err := f.keeper.Params.Get(f.ctx)
			require.NoError(t, err)
			require.Equal(t, tc.input.Params, got)
		})
	}
}
