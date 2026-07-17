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

func TestHistoricalKeyregistryInvalidArgumentFail(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetHistoricalKeyregistry(f.ctx, nil)
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestHistoricalKeyregistryNilValidatorEntry(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetHistoricalKeyregistry(f.ctx, &types.QueryGetHistoricalKeyregistryRequest{
		Validators: []*common.ValidatorEntry{nil},
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestHistoricalKeyregistryInvalidCosmosPublicKey(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetHistoricalKeyregistry(f.ctx, &types.QueryGetHistoricalKeyregistryRequest{
		Validators: []*common.ValidatorEntry{{
			ValidatorCosmosPubKey: []byte("bad-validator-cosmos-key"),
			ConsensusPower:        10,
		}},
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestHistoricalKeyregistryValidatorPubkeyNotFound(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetHistoricalKeyregistry(f.ctx, &types.QueryGetHistoricalKeyregistryRequest{
		Validators: []*common.ValidatorEntry{{
			ValidatorCosmosPubKey: generateValidatorCosmosPrivKey().PubKey().Bytes(),
			ConsensusPower:        10,
		}},
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.NotFound, st.Code())
}

func TestHistoricalKeyregistrySuccess(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	ms := keeper.NewMsgServerImpl(f.keeper)

	firstCosmosPriv := generateValidatorCosmosPrivKey()
	firstMinaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	firstCreator, firstCosmosPubKey, firstMinaPubKey, firstCosmosSig, firstMinaSig, err := signValidatorRegistration(firstCosmosPriv, firstMinaPriv)
	require.NoError(t, err)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         firstCreator,
		CosmosSignature: firstCosmosSig,
		MinaSignature:   firstMinaSig,
		CosmosPublicKey: firstCosmosPubKey,
		MinaPublicKey:   firstMinaPubKey,
		ActorType:       types.ActorType_VALIDATOR,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	secondCosmosPriv := generateValidatorCosmosPrivKey()
	secondMinaPriv, err := generateMinaSecondaryKeyPair(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	secondCreator, secondCosmosPubKey, secondMinaPubKey, secondCosmosSig, secondMinaSig, err := signValidatorRegistration(secondCosmosPriv, secondMinaPriv)
	require.NoError(t, err)

	resp, err = ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         secondCreator,
		CosmosSignature: secondCosmosSig,
		MinaSignature:   secondMinaSig,
		CosmosPublicKey: secondCosmosPubKey,
		MinaPublicKey:   secondMinaPubKey,
		ActorType:       types.ActorType_VALIDATOR,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	queryResp, err := qs.GetHistoricalKeyregistry(f.ctx, &types.QueryGetHistoricalKeyregistryRequest{
		Validators: []*common.ValidatorEntry{
			{
				ValidatorCosmosPubKey: firstCosmosPubKey,
				ConsensusPower:        10,
			},
			{
				ValidatorCosmosPubKey: secondCosmosPubKey,
				ConsensusPower:        5,
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, queryResp)
	require.Equal(t, []*types.RegisteredValidatorSetEntry{
		{
			ValidatorCosmosPubKey: firstCosmosPubKey,
			ValidatorMinaPubKey:   firstMinaPubKey,
			ConsensusPower:        10,
		},
		{
			ValidatorCosmosPubKey: secondCosmosPubKey,
			ValidatorMinaPubKey:   secondMinaPubKey,
			ConsensusPower:        5,
		},
	}, queryResp.RegisteredValidators)
}
