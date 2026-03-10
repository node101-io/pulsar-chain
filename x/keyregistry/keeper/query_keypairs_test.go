package keeper_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestCosmosMapInvalidArgumentFail verifies that GetCosmosPubKey returns
// an InvalidArgument error when called with a nil request.
func TestCosmosMapInvalidArgumentFail(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetCosmosPubKey(f.ctx, nil)
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

// TestCosmosMapSuccess verifies that a mina public key can be retrieved
// by its associated cosmos public key after being stored in the CosmosToMina map.
func TestCosmosMapSuccess(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	cosmosPriv := secp256k1.GenPrivKey()

	cosmosPubKey := cosmosPriv.PubKey()

	minaPubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	require.NoError(t, err)

	err = f.keeper.SetCosmosToMina(f.ctx, cosmosPubKey.Bytes(), minaPubKey)
	require.NoError(t, err)

	resp, err := qs.GetMinaPubKey(f.ctx, &types.QueryGetMinaPubKeyRequest{
		CosmosPubKey: cosmosPubKey.Bytes(),
	})

	require.NotNil(t, resp)
	require.NoError(t, err)

	require.Equal(t, resp.MinaPubKey, []byte(minaPubKey))
}

// TestMinaMapSuccess verifies that a cosmos public key can be retrieved
// by its associated mina public key after being stored in the MinaToCosmos map.
func TestMinaMapSuccess(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	cosmosPriv := secp256k1.GenPrivKey()

	cosmosPubKey := cosmosPriv.PubKey()

	minaPubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	require.NoError(t, err)

	err = f.keeper.SetMinaToCosmos(f.ctx, minaPubKey, cosmosPubKey.Bytes())
	require.NoError(t, err)

	resp, err := qs.GetCosmosPubKey(f.ctx, &types.QueryGetCosmosPubKeyRequest{
		MinaPubKey: minaPubKey,
	})

	require.NotNil(t, resp)
	require.NoError(t, err)

	require.Equal(t, resp.CosmosPubKey, cosmosPubKey.Bytes())
}

// TestMinaMapInvalidArgumentFail verifies that GetMinaPubKey returns
// an InvalidArgument error when called with a nil request.
func TestMinaMapInvalidArgumentFail(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetMinaPubKey(f.ctx, nil)
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

// TestCosmosMapPubkeyNotFound verifies that GetCosmosPubKey returns
// a NotFound error when the provided mina public key has no associated cosmos key.
func TestCosmosMapPubkeyNotFound(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	_, err = qs.GetCosmosPubKey(f.ctx, &types.QueryGetCosmosPubKeyRequest{
		MinaPubKey: pub,
	})

	st, _ := status.FromError(err)
	require.Equal(t, codes.NotFound, st.Code())
}

// TestMinaMapPubkeyNotFound verifies that GetMinaPubKey returns
// a NotFound error when the provided cosmos public key has no associated mina key.
func TestMinaMapPubkeyNotFound(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	priv := secp256k1.GenPrivKey()

	pub := priv.PubKey()

	_, err := qs.GetMinaPubKey(f.ctx, &types.QueryGetMinaPubKeyRequest{
		CosmosPubKey: pub.Bytes(),
	})

	st, _ := status.FromError(err)
	require.Equal(t, codes.NotFound, st.Code())
}
