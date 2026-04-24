package keeper_test

import (
	"testing"

	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestValidatorCosmosMapInvalidArgumentFail verifies that GetCosmosPubKey returns
// an InvalidArgument error when called with a nil request.
func TestValidatorCosmosMapInvalidArgumentFail(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetValidatorCosmosPubKey(f.ctx, nil)
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

// TestValidatorCosmosMapSuccess verifies that a mina public key can be retrieved
// by its associated cosmos public key after being stored in the CosmosToMina map.
func TestValidatorCosmosMapSuccess(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	cosmosPubKey, minaPubKey, _, err := generateValidatorPublicKeys()
	require.NoError(t, err)

	err = f.keeper.ValidatorSetCosmosToMina(f.ctx, cosmosPubKey.Bytes(), minaPubKey)
	require.NoError(t, err)

	resp, err := qs.GetValidatorMinaPubKey(f.ctx, &types.QueryGetValidatorMinaPubKeyRequest{
		ValidatorCosmosPubKey: cosmosPubKey.Bytes(),
	})

	require.NotNil(t, resp)
	require.NoError(t, err)

	require.Equal(t, resp.ValidatorMinaPubKey, minaPubKey)
}

// TestValidatorMinaMapSuccess verifies that a cosmos public key can be retrieved
// by its associated mina public key after being stored in the MinaToCosmos map.
func TestValidatorMinaMapSuccess(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	cosmosPubKey, minaPubKey, _, err := generateValidatorPublicKeys()
	require.NoError(t, err)

	err = f.keeper.ValidatorSetMinaToCosmos(f.ctx, minaPubKey, cosmosPubKey.Bytes())
	require.NoError(t, err)

	resp, err := qs.GetValidatorCosmosPubKey(f.ctx, &types.QueryGetValidatorCosmosPubKeyRequest{
		ValidatorMinaPubKey: minaPubKey,
	})
	require.NotNil(t, resp)
	require.NoError(t, err)

	require.Equal(t, resp.ValidatorCosmosPubKey, cosmosPubKey.Bytes())
}

// TestValidatorMinaMapInvalidArgumentFail verifies that GetMinaPubKey returns
// an InvalidArgument error when called with a nil request.
func TestValidatorMinaMapInvalidArgumentFail(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetValidatorMinaPubKey(f.ctx, nil)
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

// TestValidatorCosmosMapPubkeyNotFound verifies that GetCosmosPubKey returns
// a NotFound error when the provided mina public key has no associated cosmos key.
func TestValidatorCosmosMapPubkeyNotFound(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, minaPubKey, _, err := generateValidatorPublicKeys()
	require.NotNil(t, minaPubKey)
	require.NoError(t, err)

	_, err = qs.GetValidatorCosmosPubKey(f.ctx, &types.QueryGetValidatorCosmosPubKeyRequest{
		ValidatorMinaPubKey: minaPubKey,
	})

	st, _ := status.FromError(err)
	require.Equal(t, codes.NotFound, st.Code())
}

// TestValidatorMinaMapPubkeyNotFound verifies that GetMinaPubKey returns
// a NotFound error when the provided cosmos public key has no associated mina key.
func TestValidatorMinaMapPubkeyNotFound(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	cosmosPubKey, minaPubKey, _, err := generateValidatorPublicKeys()
	require.NotNil(t, cosmosPubKey)
	require.NotNil(t, minaPubKey)
	require.NoError(t, err)

	_, err = qs.GetValidatorMinaPubKey(f.ctx, &types.QueryGetValidatorMinaPubKeyRequest{
		ValidatorCosmosPubKey: CosmosPubKey,
	})

	st, _ := status.FromError(err)
	require.Equal(t, codes.NotFound, st.Code())
}
