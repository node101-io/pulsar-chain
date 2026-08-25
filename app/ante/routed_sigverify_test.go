package ante

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// Cosmos-auth txs should stay on the wrapped SDK signature-verification path.
// This keeps Mina verification logic isolated to Mina mode only.
func TestRoutedSigVerificationDecoratorRoutesToCosmosDecorator(t *testing.T) {
	t.Parallel()

	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeCosmos)
	delegate := &recordingDecorator{}
	minaVerifier := &recordingVerifier{}
	smartAccountVerifier := &recordingVerifier{}
	nextCalled := false

	_, err := NewRoutedSigVerificationDecorator(
		delegate,
		minaVerifier,
		smartAccountVerifier,
	).AnteHandle(
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
	require.Zero(t, minaVerifier.calls)
	require.Zero(t, smartAccountVerifier.calls)
	require.True(t, nextCalled)
}

// Mina-auth txs must be verified by the custom Mina verifier instead of the SDK decorator.
// A successful verification should continue to the next ante handler.
func TestRoutedSigVerificationDecoratorRoutesToMinaVerifier(t *testing.T) {
	t.Parallel()

	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeMina)
	delegate := &recordingDecorator{}
	minaVerifier := &recordingVerifier{}
	smartAccountVerifier := &recordingVerifier{}
	nextCalled := false

	_, err := NewRoutedSigVerificationDecorator(
		delegate,
		minaVerifier,
		smartAccountVerifier,
	).AnteHandle(
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
	require.Equal(t, 1, minaVerifier.calls)
	require.Zero(t, smartAccountVerifier.calls)
	require.True(t, nextCalled)
}

// Mina verifier failures must abort the ante chain immediately.
// This preserves the verifier error rather than masking it in routing code.
func TestRoutedSigVerificationDecoratorPropagatesMinaVerifierErrors(t *testing.T) {
	t.Parallel()

	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeMina)

	_, err := NewRoutedSigVerificationDecorator(
		&recordingDecorator{},
		&recordingVerifier{err: errVerifyFailed},
		&recordingVerifier{},
	).AnteHandle(
		ctx,
		nil,
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			t.Fatal("next handler should not be called when verifier fails")
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, errVerifyFailed)
}

func TestRoutedSigVerificationDecoratorRoutesToSmartAccountVerifier(t *testing.T) {
	t.Parallel()

	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeSmartAccount)
	delegate := &recordingDecorator{}
	minaVerifier := &recordingVerifier{}
	smartAccountVerifier := &recordingVerifier{}
	nextCalled := false

	_, err := NewRoutedSigVerificationDecorator(
		delegate,
		minaVerifier,
		smartAccountVerifier,
	).AnteHandle(
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
	require.Zero(t, minaVerifier.calls)
	require.Equal(t, 1, smartAccountVerifier.calls)
	require.True(t, nextCalled)
}

func TestRoutedSigVerificationDecoratorPropagatesSmartAccountVerifierErrors(t *testing.T) {
	t.Parallel()

	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeSmartAccount)

	_, err := NewRoutedSigVerificationDecorator(
		&recordingDecorator{},
		&recordingVerifier{},
		&recordingVerifier{err: errVerifyFailed},
	).AnteHandle(
		ctx,
		nil,
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			t.Fatal("next handler should not be called when verifier fails")
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, errVerifyFailed)
}
