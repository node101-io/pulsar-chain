package ante

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// Cosmos-auth txs must keep using the wrapped SDK decorator.
// This confirms the custom branch does not interfere outside Mina mode.
func TestRoutedSetPubKeyDecoratorRoutesToCosmosDecorator(t *testing.T) {
	t.Parallel()

	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeCosmos)
	delegate := &recordingDecorator{}
	nextCalled := false

	_, err := NewRoutedSetPubKeyDecorator(delegate).AnteHandle(
		ctx,
		nil,
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			nextCalled = true
			return nextCtx, nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, 1, delegate.calls)
	require.True(t, nextCalled)
}

func TestRoutedSetPubKeyDecoratorSkipsForSmartAccount(t *testing.T) {
	t.Parallel()

	ctx := setTxAuthMode(
		newTestSDKContext(t),
		TxAuthModeSmartAccount,
	)

	delegate := &recordingDecorator{}
	nextCalled := false

	_, err := NewRoutedSetPubKeyDecorator(delegate).AnteHandle(
		ctx,
		nil,
		false,
		func(
			nextCtx sdk.Context,
			tx sdk.Tx,
			simulate bool,
		) (sdk.Context, error) {
			nextCalled = true
			return nextCtx, nil
		},
	)

	require.NoError(t, err)
	require.Zero(t, delegate.calls)
	require.True(t, nextCalled)
}

// Mina-auth txs intentionally skip Cosmos pubkey population.
// The decorator should fall through directly to the next handler.
func TestRoutedSetPubKeyDecoratorSkipsForMina(t *testing.T) {
	t.Parallel()

	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeMina)
	delegate := &recordingDecorator{}
	nextCalled := false

	_, err := NewRoutedSetPubKeyDecorator(delegate).AnteHandle(
		ctx,
		nil,
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			nextCalled = true
			return nextCtx, nil
		},
	)

	require.NoError(t, err)
	require.Zero(t, delegate.calls)
	require.True(t, nextCalled)
}
