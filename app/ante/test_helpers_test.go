package ante

import (
	"errors"
	"testing"

	"cosmossdk.io/log"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// newTestSDKContext provides a stable default context for ante unit tests.
// Individual tests override only the flags or meters that matter for their branch.
func newTestSDKContext(tb testing.TB) sdk.Context {
	tb.Helper()

	return sdk.NewContext(nil, cmtproto.Header{
		ChainID: "pulsar-test-1",
		Height:  1,
	}, false, log.NewNopLogger())
}

// recordingDecorator tracks whether a routed decorator delegated to the wrapped SDK path.
type recordingDecorator struct {
	calls int
}

func (d *recordingDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	d.calls++
	return next(ctx, tx, simulate)
}

// recordingVerifier tracks whether a custom signature-verification branch was invoked.
type recordingVerifier struct {
	calls int
	err   error
}

func (v *recordingVerifier) VerifySignatures(ctx sdk.Context, tx sdk.Tx, simulate bool) error {
	v.calls++
	return v.err
}

var errVerifyFailed = errors.New("verify failed")
