package ante

import (
	"context"
	"errors"
	"testing"
	"time"

	"cosmossdk.io/core/address"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"
)

type validateSigCountAccountKeeper struct {
	params authtypes.Params
}

// The routed sig-count tests only need auth params, not real account storage behavior.
func (k validateSigCountAccountKeeper) GetParams(context.Context) authtypes.Params {
	if k.params.TxSigLimit == 0 {
		return authtypes.DefaultParams()
	}

	return k.params
}

func (validateSigCountAccountKeeper) GetAccount(context.Context, sdk.AccAddress) sdk.AccountI {
	return nil
}

func (validateSigCountAccountKeeper) SetAccount(context.Context, sdk.AccountI) {}

func (validateSigCountAccountKeeper) GetModuleAddress(string) sdk.AccAddress { return nil }

func (validateSigCountAccountKeeper) AddressCodec() address.Codec { return nil }

func (validateSigCountAccountKeeper) UnorderedTransactionsEnabled() bool { return false }

func (validateSigCountAccountKeeper) RemoveExpiredUnorderedNonces(sdk.Context) error { return nil }

func (validateSigCountAccountKeeper) TryAddUnorderedNonce(sdk.Context, []byte, time.Time) error {
	return nil
}

type stubBasicTx struct{}

func (stubBasicTx) GetMsgs() []sdk.Msg { return nil }

func (stubBasicTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func (stubBasicTx) ValidateBasic() error { return nil }

// stubSigVerifiableTx exposes just enough SignatureV2 behavior to drive the Mina-specific branch.
type stubSigVerifiableTx struct {
	stubBasicTx
	sigs    []signingtypes.SignatureV2
	sigsErr error
}

func (tx stubSigVerifiableTx) GetSigners() ([][]byte, error) { return nil, nil }

func (tx stubSigVerifiableTx) GetPubKeys() ([]cryptotypes.PubKey, error) { return nil, nil }

func (tx stubSigVerifiableTx) GetSignaturesV2() ([]signingtypes.SignatureV2, error) {
	if tx.sigsErr != nil {
		return nil, tx.sigsErr
	}

	return tx.sigs, nil
}

// Cosmos-auth txs must bypass Mina-specific logic and use the wrapped SDK decorator.
// This confirms the custom decorator only changes Mina-mode behavior.
func TestRoutedValidateSigCountDecoratorRoutesToCosmosDecorator(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeCosmos)
	delegate := &recordingDecorator{}
	nextCalled := false

	_, err := NewRoutedValidateSigCountDecorator(validateSigCountAccountKeeper{}, delegate).AnteHandle(
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

// Mina-auth txs must implement SigVerifiableTx because the decorator inspects SignatureV2 entries.
// A plain sdk.Tx should fail immediately before any custom counting logic runs.
func TestRoutedValidateSigCountDecoratorRejectsNonSigVerifiableTxForMina(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeMina)

	_, err := NewRoutedValidateSigCountDecorator(validateSigCountAccountKeeper{}, &recordingDecorator{}).AnteHandle(
		ctx,
		stubBasicTx{},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			t.Fatal("next handler should not be called for invalid Mina tx types")
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, sdkerrors.ErrTxDecode)
	require.ErrorContains(t, err, "Tx must be a sigTx")
}

// Signature extraction errors come from the tx itself and should bubble up unchanged.
// Stopping early here keeps routing code from masking lower-level tx problems.
func TestRoutedValidateSigCountDecoratorPropagatesGetSignaturesError(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeMina)
	expectedErr := errors.New("signatures unavailable")

	_, err := NewRoutedValidateSigCountDecorator(validateSigCountAccountKeeper{}, &recordingDecorator{}).AnteHandle(
		ctx,
		stubSigVerifiableTx{sigsErr: expectedErr},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			t.Fatal("next handler should not be called when signatures cannot be loaded")
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, expectedErr)
}

// Mina-auth txs with only single signatures and a sufficient limit should pass through.
// This is the happy path for the custom Mina-specific signature counting rules.
func TestRoutedValidateSigCountDecoratorAllowsSingleSignaturesWithinLimit(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeMina)
	nextCalled := false

	_, err := NewRoutedValidateSigCountDecorator(
		validateSigCountAccountKeeper{params: authtypes.Params{TxSigLimit: 2}},
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
}

// Mina-auth explicitly does not support multisig payloads in this chain.
// Rejecting here prevents later decorators from assuming unsupported signature structure.
func TestRoutedValidateSigCountDecoratorRejectsMultiSignatureData(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeMina)

	_, err := NewRoutedValidateSigCountDecorator(validateSigCountAccountKeeper{}, &recordingDecorator{}).AnteHandle(
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
}

// Any non-single, non-multisig SignatureV2 payload should fail fast.
// A nil payload is the smallest stable way to cover the default branch.
func TestRoutedValidateSigCountDecoratorRejectsUnexpectedSignatureDataType(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeMina)

	_, err := NewRoutedValidateSigCountDecorator(validateSigCountAccountKeeper{}, &recordingDecorator{}).AnteHandle(
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
}

// The decorator must enforce the chain TxSigLimit even on the Mina path.
// Exceeding the limit should stop the ante chain before any later verification runs.
func TestRoutedValidateSigCountDecoratorRejectsSignatureCountAboveLimit(t *testing.T) {
	t.Parallel()
	ctx := setTxAuthMode(newTestSDKContext(t), TxAuthModeMina)

	_, err := NewRoutedValidateSigCountDecorator(
		validateSigCountAccountKeeper{params: authtypes.Params{TxSigLimit: 1}},
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
			t.Fatal("next handler should not be called when the signature limit is exceeded")
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, sdkerrors.ErrTooManySignatures)
	require.ErrorContains(t, err, "signatures: 2, limit: 1")
}
