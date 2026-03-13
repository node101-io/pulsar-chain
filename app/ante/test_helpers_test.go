package ante

import (
	"testing"

	"cosmossdk.io/log"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func newTestSDKContext(tb testing.TB) sdk.Context {
	tb.Helper()

	return sdk.NewContext(nil, cmtproto.Header{
		ChainID: "pulsar-test-1",
		Height:  1,
	}, false, log.NewNopLogger())
}
