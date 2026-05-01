package keeper_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

// TestUserCosmosMapSuccess verifies that a mina public key can be retrieved
// by its associated cosmos public key after being stored in the CosmosToMina map.
func TestUserCosmosMapSuccess(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPublicKey, minaPubKey, _, err := generateUserPublicKeys()
	require.NoError(t, err)
	require.NotNil(t, cosmosPublicKey)
	require.NotNil(t, minaPubKey)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPublicKey.Bytes(),
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	queryResp, err := qs.GetUserMinaPublicKey(f.ctx, &types.QueryGetUserMinaPublicKeyRequest{
		UserCosmosPublicKey: cosmosPublicKey.Bytes(),
	})

	require.NotNil(t, queryResp)
	require.NoError(t, err)

	require.Equal(t, minaPubKey, queryResp.UserMinaPublicKey)
}

// TestUserMinaMapSuccess verifies that a cosmos public key can be retrieved
// by its associated mina public key after being stored in the MinaToCosmos map.
func TestUserMinaMapSuccess(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	ms := keeper.NewMsgServerImpl(f.keeper)

	cosmosPublicKey, minaPubKey, _, err := generateUserPublicKeys()
	require.NoError(t, err)
	require.NotNil(t, cosmosPublicKey)
	require.NotNil(t, minaPubKey)

	creatorAddr := sdk.AccAddress(cosmosPublicKey.Address())
	require.NotNil(t, creatorAddr)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: mockCosmosSignature,
		MinaSignature:   mockMinaSignature,
		CosmosPublicKey: cosmosPublicKey.Bytes(),
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_USER,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	queryResp, err := qs.GetUserCosmosPublicKey(f.ctx, &types.QueryGetUserCosmosPublicKeyRequest{
		UserMinaPublicKey: minaPubKey,
	})

	require.NotNil(t, queryResp)
	require.NoError(t, err)

	require.Equal(t, cosmosPublicKey.Bytes(), queryResp.UserCosmosPublicKey)
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

// TestUserCosmosMapPubkeyNotFound verifies that GetCosmosPubKey returns
// a NotFound error when the provided mina public key has no associated cosmos key.
func TestUserCosmosMapPubkeyNotFound(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	_, err = qs.GetUserCosmosPublicKey(f.ctx, &types.QueryGetUserCosmosPublicKeyRequest{
		UserMinaPublicKey: pub,
	})

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

	st, _ := status.FromError(err)
	require.Equal(t, codes.NotFound, st.Code())
}
