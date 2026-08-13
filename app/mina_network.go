package app

import (
	"fmt"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
)

func normalizeMinaNetworkID(networkID string) (mina.NetworkID, error) {
	switch networkID {
	case "devnet", string(mina.TestNet):
		return mina.TestNet, nil
	case string(mina.MainNet):
		return mina.MainNet, nil
	default:
		return "", fmt.Errorf("unsupported mina network ID %q", networkID)
	}
}
