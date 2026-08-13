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

func TestValidatorCosmosMapInvalidMinaPublicKey(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetValidatorCosmosPubKey(f.ctx, &types.QueryGetValidatorCosmosPubKeyRequest{
		ValidatorMinaPubKey: malformedMinaPublicKey(),
	})
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

	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateValidatorCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	msg := newValidatorRegistration(t, f, cosmosPriv, minaPriv)
	resp, err := ms.RegisterValidatorKeys(f.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	queryResp, err := qs.GetValidatorMinaPubKey(f.ctx, &types.QueryGetValidatorMinaPubKeyRequest{
		ValidatorCosmosPubKey: msg.ValidatorConsensusPublicKey,
	})
	require.NoError(t, err)
	require.NotNil(t, queryResp)
	require.Equal(t, msg.MinaPublicKey, queryResp.ValidatorMinaPubKey)
}

// TestValidatorMinaMapSuccess verifies that a cosmos public key can be retrieved
// by its associated mina public key after being stored in the MinaToCosmos map.
func TestValidatorMinaMapSuccess(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateValidatorCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)

	msg := newValidatorRegistration(t, f, cosmosPriv, minaPriv)
	resp, err := ms.RegisterValidatorKeys(f.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	queryResp, err := qs.GetValidatorCosmosPubKey(f.ctx, &types.QueryGetValidatorCosmosPubKeyRequest{
		ValidatorMinaPubKey: msg.MinaPublicKey,
	})
	require.NoError(t, err)
	require.NotNil(t, queryResp)
	require.Equal(t, msg.ValidatorConsensusPublicKey, queryResp.ValidatorCosmosPubKey)
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

func TestValidatorMinaMapInvalidCosmosPublicKey(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetValidatorMinaPubKey(f.ctx, &types.QueryGetValidatorMinaPubKeyRequest{
		ValidatorCosmosPubKey: []byte("bad-validator-cosmos-key"),
	})
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

	minaPriv, err := generateMinaKey(types.ActorType_VALIDATOR)
	require.NoError(t, err)
	require.NotNil(t, minaPriv)

	minaPubKey, err := minaPriv.ToPublicKey()
	require.NoError(t, err)
	require.NotNil(t, minaPubKey)

	_, err = qs.GetValidatorCosmosPubKey(f.ctx, &types.QueryGetValidatorCosmosPubKeyRequest{
		ValidatorMinaPubKey: minaPubKey.Bytes(),
	})
	require.Error(t, err)

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

	cosmosPriv := generateValidatorCosmosPrivKey()
	cosmosPubKey := cosmosPriv.PubKey()
	require.NotNil(t, cosmosPubKey)

	_, err := qs.GetValidatorMinaPubKey(f.ctx, &types.QueryGetValidatorMinaPubKeyRequest{
		ValidatorCosmosPubKey: cosmosPubKey.Bytes(),
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.NotFound, st.Code())
}
