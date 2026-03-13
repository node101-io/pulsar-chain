package vote_ext

import (
	"encoding/base64"
	"fmt"
	"math/big"

	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

func GetSecondaryKeys(appOpts servertypes.AppOptions) types.SecondaryKey {
	minaPrivKey := appOpts.Get("vote_extension.priv_key")
	keyStr, ok := minaPrivKey.(string)
	if !ok {
		panic("vote_extension.priv_key is not a string")
	}

	// Decode base64 -> bytes
	keyBytes, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		panic(fmt.Sprintf("failed to decode base64 priv key: %v", err))
	}

	// Bytes -> big.Int
	prv := new(big.Int).SetBytes(keyBytes)
	priv := keys.PrivateKey{
		Value: prv,
	}
	public := priv.ToPublicKey()
	secondaryKey := types.SecondaryKey{
		SecretKey: &priv,
		PublicKey: &public,
	}

	return secondaryKey
}
