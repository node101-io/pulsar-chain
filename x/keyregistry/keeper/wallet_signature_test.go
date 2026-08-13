package keeper_test

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/publickey"
	"github.com/node101-io/mina-signer-go/signature"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

const (
	legacyWalletMinaPublicKeyB64 = "7fW+TcYCvStQ0ZYv1arI4MUF/aQ38xqc4TQfMokuvJs="
	legacyWalletUserSignatureB64 = "EN17Tzl+OTmGSg6L3tp0SgGD3xi2U2L0cWSh9qR2JBrrt1/+Qkm4pfLrMhVpdmSM42GDHeucmDb3rj/dMVsvAw=="
	legacyWalletCosmosKeyHex     = "028e23b60777010732ad6bc2607f5ee5624fbba62ad284bc1300852cf90b2d94b0"
)

// Real Auro signFields output over a REGISTER challenge for chain
// "mytestnet". Auro returned the same bytes on mina:devnet, mina:mainnet and
// zeko:testnet, so the connected network adds no domain separation.
// Signature is field then scalar, each little-endian.
const (
	auroWalletMinaPublicKeyB64 = "7fW+TcYCvStQ0ZYv1arI4MUF/aQ38xqc4TQfMokuvJs="
	auroWalletUserSignatureB64 = "089lFXPNS+BrrgeqNusaYFe+2ie/DJkvVbPuNrSuIAY8N9j/sqsA0xEr5XgSeF9uEUUBzfg3Opr/EeQfib69Pg=="
	auroWalletCosmosKeyHex     = "028e23b60777010732ad6bc2607f5ee5624fbba62ad284bc1300852cf90b2d94b0"
	auroWalletChainID          = "mytestnet"
)

func auroWalletFixture(t *testing.T) (cosmosPublicKey, minaPublicKey, minaSignature []byte, creator string) {
	t.Helper()

	cosmosPublicKey, err := hex.DecodeString(auroWalletCosmosKeyHex)
	require.NoError(t, err)
	minaPublicKey, err = base64.StdEncoding.DecodeString(auroWalletMinaPublicKeyB64)
	require.NoError(t, err)
	minaSignature, err = base64.StdEncoding.DecodeString(auroWalletUserSignatureB64)
	require.NoError(t, err)

	creator = sdk.AccAddress((&secp256k1.PubKey{Key: cosmosPublicKey}).Address()).String()

	return cosmosPublicKey, minaPublicKey, minaSignature, creator
}

func TestLegacyAuroRegistrationProofCannotAuthorizeV2Challenge(t *testing.T) {
	f := initFixture(t)
	cosmosPublicKey, err := hex.DecodeString(legacyWalletCosmosKeyHex)
	require.NoError(t, err)
	minaPublicKey, err := base64.StdEncoding.DecodeString(legacyWalletMinaPublicKeyB64)
	require.NoError(t, err)
	minaSignature, err := base64.StdEncoding.DecodeString(legacyWalletUserSignatureB64)
	require.NoError(t, err)
	cosmosKey := &secp256k1.PubKey{Key: cosmosPublicKey}
	creator := sdk.AccAddress(cosmosKey.Address()).String()

	_, err = keeper.NewMsgServerImpl(f.keeper).RegisterUserKeys(f.ctx, &types.MsgRegisterUserKeys{
		Creator: creator, CosmosPublicKey: cosmosPublicKey, MinaPublicKey: minaPublicKey, MinaSignature: minaSignature,
	})
	require.ErrorIs(t, err, types.ErrInvalidSignature)
}

func TestAuroWalletProofRegistersUser(t *testing.T) {
	f := initFixture(t)
	cosmosPublicKey, minaPublicKey, minaSignature, creator := auroWalletFixture(t)
	ctx := sdk.UnwrapSDKContext(f.ctx).WithChainID(auroWalletChainID)

	_, err := keeper.NewMsgServerImpl(f.keeper).RegisterUserKeys(ctx, &types.MsgRegisterUserKeys{
		Creator: creator, CosmosPublicKey: cosmosPublicKey, MinaPublicKey: minaPublicKey, MinaSignature: minaSignature,
	})
	require.NoError(t, err)

	storedMina, err := f.keeper.UserGetCosmosToMina(ctx, cosmosPublicKey)
	require.NoError(t, err)
	require.Equal(t, minaPublicKey, storedMina)
}

// initFixture runs on "pulsar-test-1", so the proof lands on the wrong chain.
func TestAuroWalletProofIsRejectedOnAnotherChain(t *testing.T) {
	f := initFixture(t)
	cosmosPublicKey, minaPublicKey, minaSignature, creator := auroWalletFixture(t)

	_, err := keeper.NewMsgServerImpl(f.keeper).RegisterUserKeys(f.ctx, &types.MsgRegisterUserKeys{
		Creator: creator, CosmosPublicKey: cosmosPublicKey, MinaPublicKey: minaPublicKey, MinaSignature: minaSignature,
	})
	require.ErrorIs(t, err, types.ErrInvalidSignature)
}

// walletFieldSignatureNetworkID assumes TestNet; MainNet must fail.
func TestAuroWalletProofVerifiesOnlyUnderTestNetDomain(t *testing.T) {
	cosmosPublicKey, minaPublicKey, minaSignature, _ := auroWalletFixture(t)

	challenge, err := types.BuildKeySigningChallenge(types.KeySigningChallengeInput{
		ChainID:          auroWalletChainID,
		Operation:        types.KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER,
		ActorType:        types.ActorType_USER,
		CosmosPublicKey:  cosmosPublicKey,
		NewMinaPublicKey: minaPublicKey,
	})
	require.NoError(t, err)

	sig, err := signature.NewSignatureFromBytes(minaSignature)
	require.NoError(t, err)

	for _, tc := range []struct {
		networkID mina.NetworkID
		want      bool
	}{
		{mina.TestNet, true},
		{mina.DevNet, true},
		{mina.MainNet, false},
	} {
		publicKey, err := publickey.NewPublicKeyFromBytes(minaPublicKey, tc.networkID)
		require.NoError(t, err)

		// A bad signature errors rather than returning false.
		valid, err := publicKey.VerifyField(sig, challenge)
		require.Equal(t, tc.want, err == nil && valid, "network %s", tc.networkID)
	}
}
