package ante

import (
	"errors"
	"testing"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"
)

var errSigGasTestSignaturesUnavailable = errors.New("signatures unavailable")

// newGasMeteredContext installs a bounded gas meter so tests can assert on gas deltas.
// The default test context uses an infinite meter, which is not useful for accounting checks.
func newGasMeteredContext(t *testing.T) sdk.Context {
	t.Helper()

	return newTestSDKContext(t).WithGasMeter(storetypes.NewGasMeter(1_000_000))
}

// Cosmos-auth txs must stay on the wrapped SDK gas-accounting path.
// This confirms our custom decorator only changes Mina-mode gas accounting.
func TestRoutedSigGasConsumeDecoratorRoutesToCosmosDecorator(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newGasMeteredContext(t), TxAuthModeCosmos)
	delegate := &recordingDecorator{}
	nextCalled := false

	_, err := NewRoutedSigGasConsumeDecorator(validateSigCountAccountKeeper{}, delegate).AnteHandle(
		ctx,
		stubBasicTx{},
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

// Mina-auth gas accounting requires SignatureV2 access from SigVerifiableTx.
// Plain sdk.Tx values should fail before any gas is consumed or delegated state changes happen.
func TestRoutedSigGasConsumeDecoratorRejectsNonSigVerifiableTxForMina(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newGasMeteredContext(t), TxAuthModeMina)

	_, err := NewRoutedSigGasConsumeDecorator(validateSigCountAccountKeeper{}, &recordingDecorator{}).AnteHandle(
		ctx,
		stubBasicTx{},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			t.Fatal("next handler should not be called for invalid Mina tx types")
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, sdkerrors.ErrTxDecode)
	require.ErrorContains(t, err, "invalid transaction type")
	require.Zero(t, ctx.GasMeter().GasConsumed())
}

// Signature loading failures belong to the tx and should bubble up unchanged.
// Failing before gas consumption keeps accounting aligned with actual verification work.
func TestRoutedSigGasConsumeDecoratorPropagatesGetSignaturesError(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newGasMeteredContext(t), TxAuthModeMina)
	expectedErr := errSigGasTestSignaturesUnavailable

	_, err := NewRoutedSigGasConsumeDecorator(validateSigCountAccountKeeper{}, &recordingDecorator{}).AnteHandle(
		ctx,
		stubSigVerifiableTx{sigsErr: expectedErr},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			t.Fatal("next handler should not be called when signatures cannot be loaded")
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, expectedErr)
	require.Zero(t, ctx.GasMeter().GasConsumed())
}

// Each Mina single signature should consume the configured verify gas cost.
// We assert on the delta because the absolute meter value is an implementation detail.
func TestRoutedSigGasConsumeDecoratorConsumesGasForSingleSignatures(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newGasMeteredContext(t), TxAuthModeMina)
	nextCalled := false
	params := authtypes.DefaultParams()
	params.SigVerifyCostSecp256k1 = 37
	before := ctx.GasMeter().GasConsumed()

	_, err := NewRoutedSigGasConsumeDecorator(
		validateSigCountAccountKeeper{params: params},
		&recordingDecorator{},
	).AnteHandle(
		ctx,
		stubSigVerifiableTx{
			sigs: []signingtypes.SignatureV2{
				{Data: &signingtypes.SingleSignatureData{}},
				{Data: &signingtypes.SingleSignatureData{}},
			},
		},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			nextCalled = true
			return nextCtx, nil
		},
	)

	require.NoError(t, err)
	require.True(t, nextCalled)
	// Only the verification work added by this decorator matters, so we compare before/after.
	require.Equal(t, uint64(2*params.SigVerifyCostSecp256k1), ctx.GasMeter().GasConsumed()-before)
}

// An empty signature list is still a valid zero-iteration path for this decorator.
// This locks down the "no signatures to charge" branch without inventing special-case logic.
func TestRoutedSigGasConsumeDecoratorConsumesGasForEmptySignatureList(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newGasMeteredContext(t), TxAuthModeMina)
	nextCalled := false
	before := ctx.GasMeter().GasConsumed()

	_, err := NewRoutedSigGasConsumeDecorator(validateSigCountAccountKeeper{}, &recordingDecorator{}).AnteHandle(
		ctx,
		stubSigVerifiableTx{sigs: nil},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			nextCalled = true
			return nextCtx, nil
		},
	)

	require.NoError(t, err)
	require.True(t, nextCalled)
	require.Equal(t, before, ctx.GasMeter().GasConsumed())
}

// Mina-auth explicitly rejects multisig payloads in the gas-accounting stage too.
// This keeps gas accounting aligned with the same signature-shape restrictions as verification.
func TestRoutedSigGasConsumeDecoratorRejectsMultiSignatureData(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newGasMeteredContext(t), TxAuthModeMina)

	_, err := NewRoutedSigGasConsumeDecorator(validateSigCountAccountKeeper{}, &recordingDecorator{}).AnteHandle(
		ctx,
		stubSigVerifiableTx{
			sigs: []signingtypes.SignatureV2{
				{Data: &signingtypes.MultiSignatureData{}},
			},
		},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			t.Fatal("next handler should not be called for multisig Mina transactions")
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, sdkerrors.ErrInvalidType)
	require.ErrorContains(t, err, "mina transactions do not support multisig signatures")
	require.Zero(t, ctx.GasMeter().GasConsumed())
}

// Any unsupported SignatureV2 payload should stop gas accounting immediately.
// A nil payload is the smallest stable input for the default branch.
func TestRoutedSigGasConsumeDecoratorRejectsUnexpectedSignatureDataType(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newGasMeteredContext(t), TxAuthModeMina)

	_, err := NewRoutedSigGasConsumeDecorator(validateSigCountAccountKeeper{}, &recordingDecorator{}).AnteHandle(
		ctx,
		stubSigVerifiableTx{
			sigs: []signingtypes.SignatureV2{
				{Data: nil},
			},
		},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			t.Fatal("next handler should not be called for unsupported signature payloads")
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, sdkerrors.ErrInvalidType)
	require.ErrorContains(t, err, "unexpected signature data type <nil>")
	require.Zero(t, ctx.GasMeter().GasConsumed())
}

// Gas should be charged for valid signatures processed before the first invalid one.
// This confirms the decorator accounts for completed work even when it later aborts.
func TestRoutedSigGasConsumeDecoratorStopsGasAccountingAtFirstInvalidSignature(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newGasMeteredContext(t), TxAuthModeMina)
	params := authtypes.DefaultParams()
	params.SigVerifyCostSecp256k1 = 41
	before := ctx.GasMeter().GasConsumed()

	_, err := NewRoutedSigGasConsumeDecorator(
		validateSigCountAccountKeeper{params: params},
		&recordingDecorator{},
	).AnteHandle(
		ctx,
		stubSigVerifiableTx{
			sigs: []signingtypes.SignatureV2{
				{Data: &signingtypes.SingleSignatureData{}},
				{Data: &signingtypes.MultiSignatureData{}},
			},
		},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			t.Fatal("next handler should not be called after an invalid signature payload")
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, sdkerrors.ErrInvalidType)
	require.ErrorContains(t, err, "mina transactions do not support multisig signatures")
	// The second signature is invalid, so only the first one should have consumed verification gas.
	require.Equal(t, uint64(params.SigVerifyCostSecp256k1), ctx.GasMeter().GasConsumed()-before)
}
