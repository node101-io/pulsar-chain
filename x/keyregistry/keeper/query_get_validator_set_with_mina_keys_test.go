package keeper_test

import (
	"testing"

	common "github.com/node101-io/pulsar-chain/common"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetValidatorSetWithMinaKeysInvalidArgumentFail(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetValidatorSetWithMinaKeys(f.ctx, nil)
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGetValidatorSetWithMinaKeysNilValidatorEntry(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetValidatorSetWithMinaKeys(f.ctx, &types.QueryGetValidatorSetWithMinaKeysRequest{
		Validators: []*common.ValidatorEntry{nil},
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGetValidatorSetWithMinaKeysInvalidCosmosPublicKey(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetValidatorSetWithMinaKeys(f.ctx, &types.QueryGetValidatorSetWithMinaKeysRequest{
		Validators: []*common.ValidatorEntry{{
			ValidatorCosmosPubKey: []byte("bad-validator-cosmos-key"),
			ConsensusPower:        10,
		}},
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGetValidatorSetWithMinaKeysValidatorPubkeyNotFound(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetValidatorSetWithMinaKeys(f.ctx, &types.QueryGetValidatorSetWithMinaKeysRequest{
		Validators: []*common.ValidatorEntry{{
			ValidatorCosmosPubKey: generateValidatorCosmosPrivKey().PubKey().Bytes(),
			ConsensusPower:        10,
		}},
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.NotFound, st.Code())
}

func TestGetValidatorSetWithMinaKeysSuccess(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	ms := keeper.NewMsgServerImpl(f.keeper)

	firstCosmosPriv := generateValidatorCosmosPrivKey()
	firstMinaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	firstMsg := newValidatorRegistration(t, f, firstCosmosPriv, firstMinaPriv)
	resp, err := ms.RegisterValidatorKeys(f.ctx, firstMsg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	secondCosmosPriv := generateValidatorCosmosPrivKey()
	secondMinaPriv, err := generateMinaSecondaryKeyPair(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	secondMsg := newValidatorRegistration(t, f, secondCosmosPriv, secondMinaPriv)
	resp, err = ms.RegisterValidatorKeys(f.ctx, secondMsg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	queryResp, err := qs.GetValidatorSetWithMinaKeys(f.ctx, &types.QueryGetValidatorSetWithMinaKeysRequest{
		Validators: []*common.ValidatorEntry{
			{
				ValidatorCosmosPubKey: firstMsg.ValidatorConsensusPublicKey,
				ConsensusPower:        10,
			},
			{
				ValidatorCosmosPubKey: secondMsg.ValidatorConsensusPublicKey,
				ConsensusPower:        5,
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, queryResp)
	require.Equal(t, []*types.RegisteredValidatorSetEntry{
		{
			ValidatorCosmosPubKey: firstMsg.ValidatorConsensusPublicKey,
			ValidatorMinaPubKey:   firstMsg.MinaPublicKey,
			ConsensusPower:        10,
		},
		{
			ValidatorCosmosPubKey: secondMsg.ValidatorConsensusPublicKey,
			ValidatorMinaPubKey:   secondMsg.MinaPublicKey,
			ConsensusPower:        5,
		},
	}, queryResp.RegisteredValidators)
}
