package ante

import (
	"errors"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

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

type recordingVerifier struct {
	calls int
	err   error
}

func (v *recordingVerifier) VerifySignatures(ctx sdk.Context, tx sdk.Tx, simulate bool) error {
	v.calls++
	return v.err
}

func TestRoutedSetPubKeyDecoratorRoutesToCosmosDecorator(t *testing.T) {
	t.Parallel()

	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeCosmos)
	delegate := &recordingDecorator{}
	nextCalled := false

	_, err := NewRoutedSetPubKeyDecorator(delegate).AnteHandle(ctx, nil, false, func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		nextCalled = true
		return nextCtx, nil
	})

	require.NoError(t, err)
	require.Equal(t, 1, delegate.calls)
	require.True(t, nextCalled)
}

func TestRoutedSetPubKeyDecoratorSkipsForMina(t *testing.T) {
	t.Parallel()

	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeMina)
	delegate := &recordingDecorator{}
	nextCalled := false

	_, err := NewRoutedSetPubKeyDecorator(delegate).AnteHandle(ctx, nil, false, func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		nextCalled = true
		return nextCtx, nil
	})

	require.NoError(t, err)
	require.Zero(t, delegate.calls)
	require.True(t, nextCalled)
}

func TestRoutedSigVerificationDecoratorRoutesToCosmosDecorator(t *testing.T) {
	t.Parallel()

	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeCosmos)
	delegate := &recordingDecorator{}
	verifier := &recordingVerifier{}
	nextCalled := false

	_, err := NewRoutedSigVerificationDecorator(delegate, verifier).AnteHandle(ctx, nil, false, func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		nextCalled = true
		return nextCtx, nil
	})

	require.NoError(t, err)
	require.Equal(t, 1, delegate.calls)
	require.Zero(t, verifier.calls)
	require.True(t, nextCalled)
}

func TestRoutedSigVerificationDecoratorRoutesToMinaVerifier(t *testing.T) {
	t.Parallel()

	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeMina)
	delegate := &recordingDecorator{}
	verifier := &recordingVerifier{}
	nextCalled := false

	_, err := NewRoutedSigVerificationDecorator(delegate, verifier).AnteHandle(ctx, nil, false, func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		nextCalled = true
		return nextCtx, nil
	})

	require.NoError(t, err)
	require.Zero(t, delegate.calls)
	require.Equal(t, 1, verifier.calls)
	require.True(t, nextCalled)
}

func TestRoutedSigVerificationDecoratorPropagatesVerifierErrors(t *testing.T) {
	t.Parallel()

	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeMina)
	expectedErr := errors.New("verify failed")

	_, err := NewRoutedSigVerificationDecorator(&recordingDecorator{}, &recordingVerifier{err: expectedErr}).AnteHandle(
		ctx,
		nil,
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			t.Fatal("next handler should not be called when verifier fails")
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, expectedErr)
}
