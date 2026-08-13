package keeper_test

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

const (
	legacyWalletMinaPublicKeyB64 = "7fW+TcYCvStQ0ZYv1arI4MUF/aQ38xqc4TQfMokuvJs="
	legacyWalletUserSignatureB64 = "EN17Tzl+OTmGSg6L3tp0SgGD3xi2U2L0cWSh9qR2JBrrt1/+Qkm4pfLrMhVpdmSM42GDHeucmDb3rj/dMVsvAw=="
	legacyWalletCosmosKeyHex     = "028e23b60777010732ad6bc2607f5ee5624fbba62ad284bc1300852cf90b2d94b0"
)

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

// TODO: Add real Auro devnet, testnet, and mainnet signFields fixtures for the
// V2 challenge before merging. The mainnet fixture must verify through the
// fixed TestNet field-signature domain used by o1js and Auro.
