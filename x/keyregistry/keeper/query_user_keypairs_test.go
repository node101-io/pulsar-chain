package keeper_test

import (
	"testing"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestUserCosmosMapInvalidArgumentFail verifies that GetCosmosPubKey returns
// an InvalidArgument error when called with a nil request.
func TestUserCosmosMapInvalidArgumentFail(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetUserCosmosPublicKey(f.ctx, nil)
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestUserCosmosMapInvalidMinaPublicKey(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetUserCosmosPublicKey(f.ctx, &types.QueryGetUserCosmosPublicKeyRequest{
		UserMinaPublicKey: malformedMinaPublicKey(),
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

// TestUserCosmosMapSuccess verifies that a mina public key can be retrieved
// by its associated cosmos public key after being stored in the CosmosToMina map.
func TestUserCosmosMapSuccess(t *testing.T) {

	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateUserCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_ACTOR_TYPE_USER)
	require.NoError(t, err)

	msg := newUserRegistration(t, f.ctx, cosmosPriv, minaPriv)
	resp, err := ms.RegisterUserKeys(f.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	queryResp, err := qs.GetUserMinaPublicKey(f.ctx, &types.QueryGetUserMinaPublicKeyRequest{
		UserCosmosPublicKey: msg.CosmosPublicKey,
	})
	require.NoError(t, err)
	require.NotNil(t, queryResp)

	require.Equal(t, msg.MinaPublicKey, queryResp.UserMinaPublicKey)
}

// TestUserMinaMapSuccess verifies that a cosmos public key can be retrieved
// by its associated mina public key after being stored in the MinaToCosmos map.
func TestUserMinaMapSuccess(t *testing.T) {

	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPriv := generateUserCosmosPrivKey()
	minaPriv, err := generateMinaKey(types.ActorType_ACTOR_TYPE_USER)
	require.NoError(t, err)

	msg := newUserRegistration(t, f.ctx, cosmosPriv, minaPriv)
	resp, err := ms.RegisterUserKeys(f.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	queryResp, err := qs.GetUserCosmosPublicKey(f.ctx, &types.QueryGetUserCosmosPublicKeyRequest{
		UserMinaPublicKey: msg.MinaPublicKey,
	})
	require.NoError(t, err)
	require.NotNil(t, queryResp)

	require.Equal(t, msg.CosmosPublicKey, queryResp.UserCosmosPublicKey)
}

// TestUserMinaMapInvalidArgumentFail verifies that GetMinaPubKey returns
// an InvalidArgument error when called with a nil request.
func TestUserMinaMapInvalidArgumentFail(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetUserMinaPublicKey(f.ctx, nil)
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestUserMinaMapInvalidCosmosPublicKey(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetUserMinaPublicKey(f.ctx, &types.QueryGetUserMinaPublicKeyRequest{
		UserCosmosPublicKey: []byte("bad-cosmos-key"),
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

// TestUserCosmosMapPubkeyNotFound verifies that GetCosmosPubKey returns
// a NotFound error when the provided mina public key has no associated cosmos key.
func TestUserCosmosMapPubkeyNotFound(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	minaPriv, err := generateMinaKey(types.ActorType_ACTOR_TYPE_USER)
	require.NoError(t, err)

	minaPubKey, err := minaPriv.ToPublicKey()
	require.NoError(t, err)

	_, err = qs.GetUserCosmosPublicKey(f.ctx, &types.QueryGetUserCosmosPublicKeyRequest{
		UserMinaPublicKey: minaPubKey.Bytes(),
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.NotFound, st.Code())
}

// TestUserMinaMapPubkeyNotFound verifies that GetMinaPubKey returns
// a NotFound error when the provided cosmos public key has no associated mina key.
func TestUserMinaMapPubkeyNotFound(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	priv := secp256k1.GenPrivKey()

	pub := priv.PubKey()

	_, err := qs.GetUserMinaPublicKey(f.ctx, &types.QueryGetUserMinaPublicKeyRequest{
		UserCosmosPublicKey: pub.Bytes(),
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.NotFound, st.Code())
}
