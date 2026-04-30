package main

import (
	"encoding/base64"
	"fmt"
	"math/big"
	"os"

	"github.com/node101-io/mina-signer-go/keys"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: derive_mina_pub.go <mina-private-key-base64>")
	}

	privB64 := os.Args[1]
	keyBytes, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil {
		panic(err)
	}

	priv := keys.PrivateKey{Value: new(big.Int).SetBytes(keyBytes)}
	pub := priv.ToPublicKey()
	bz, err := pub.Marshal()
	if err != nil {
		panic(err)
	}

	fmt.Print(base64.StdEncoding.EncodeToString(bz))
}
