package ante

import (
	"testing"

	"cosmossdk.io/log"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	antetypes "github.com/node101-io/pulsar-chain/app/ante/types"
)

type stubExtensionTx struct {
	extensionOptions []*codectypes.Any
}

func (tx stubExtensionTx) GetMsgs() []sdk.Msg { return nil }

func (tx stubExtensionTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func (tx stubExtensionTx) GetExtensionOptions() []*codectypes.Any {
	return tx.extensionOptions
}

// mustTxAuthModeExtensionAny builds the exact Any payload consumed by the resolver.
// Using real proto bytes keeps these tests close to the on-wire tx representation.
func mustTxAuthModeExtensionAny(t *testing.T, mode antetypes.TxAuthMode) *codectypes.Any {
	t.Helper()

	bz, err := gogoproto.Marshal(&antetypes.TxAuthModeExtension{TxAuthMode: mode})
	require.NoError(t, err)

	return &codectypes.Any{
		TypeUrl: txAuthModeExtensionTypeURL,
		Value:   bz,
	}
}

// Txs without the custom extension should stay on the default Cosmos path.
// This preserves compatibility with normal SDK transactions.
func TestResolveTxAuthModeDefaultsToCosmos(t *testing.T) {
	t.Parallel()

	mode, err := ResolveTxAuthMode(stubExtensionTx{})

	require.NoError(t, err)
	require.Equal(t, TxAuthModeCosmos, mode)
}

// A Mina extension must flip resolution into Mina mode.
// This is the primary positive branch for custom auth-mode routing.
func TestResolveTxAuthModeReadsMinaExtension(t *testing.T) {
	t.Parallel()

	mode, err := ResolveTxAuthMode(stubExtensionTx{
		extensionOptions: []*codectypes.Any{
			mustTxAuthModeExtensionAny(t, antetypes.TX_AUTH_MODE_MINA),
		},
	})

	require.NoError(t, err)
	require.Equal(t, TxAuthModeMina, mode)
}

// Malformed extension bytes should surface a decoding error instead of being ignored.
// Returning an error here protects the ante chain from ambiguous tx metadata.
func TestResolveTxAuthModeRejectsMalformedExtension(t *testing.T) {
	t.Parallel()

	mode, err := ResolveTxAuthMode(stubExtensionTx{
		extensionOptions: []*codectypes.Any{
			{TypeUrl: txAuthModeExtensionTypeURL, Value: []byte("not-proto")},
		},
	})

	require.ErrorContains(t, err, "invalid tx auth mode extension")
	require.Equal(t, TxAuthModeCosmos, mode)
}

// Multiple auth-mode extensions are ambiguous and must be rejected.
// This prevents a single tx from carrying conflicting auth instructions.
func TestResolveTxAuthModeRejectsMultipleExtensions(t *testing.T) {
	t.Parallel()

	mode, err := ResolveTxAuthMode(stubExtensionTx{
		extensionOptions: []*codectypes.Any{
			mustTxAuthModeExtensionAny(t, antetypes.TX_AUTH_MODE_COSMOS),
			mustTxAuthModeExtensionAny(t, antetypes.TX_AUTH_MODE_MINA),
		},
	})

	require.ErrorContains(t, err, "multiple tx auth mode extensions found")
	require.Equal(t, TxAuthModeCosmos, mode)
}

// The extension checker must accept our custom type while still delegating all other checks.
// That keeps the custom wiring composable with any additional extension checkers above it.
func TestNewTxAuthExtensionOptionCheckerAcceptsCustomType(t *testing.T) {
	t.Parallel()

	delegated := false
	checker := NewTxAuthExtensionOptionChecker(func(extOption *codectypes.Any) bool {
		delegated = true
		return extOption != nil && extOption.TypeUrl == "/example.Extension"
	})

	require.True(t, checker(mustTxAuthModeExtensionAny(t, antetypes.TX_AUTH_MODE_MINA)))
	require.True(t, checker(&codectypes.Any{TypeUrl: "/example.Extension"}))
	require.True(t, delegated)
	require.False(t, checker(&codectypes.Any{TypeUrl: "/other.Extension"}))
}

// The decorator should resolve auth mode once and store it on the context.
// Downstream routed decorators rely on this cached value instead of reparsing the tx.
func TestTxAuthModeDecoratorStoresResolvedMode(t *testing.T) {
	t.Parallel()

	ctx := newTestSDKContext(t)
	tx := stubExtensionTx{
		extensionOptions: []*codectypes.Any{
			mustTxAuthModeExtensionAny(t, antetypes.TX_AUTH_MODE_MINA),
		},
	}

	decorator := NewTxAuthModeDecorator()
	nextCalled := false

	next := func(nextCtx sdk.Context, nextTx sdk.Tx, simulate bool) (sdk.Context, error) {
		// This is the observable contract of the decorator: downstream handlers can read the mode from context.
		nextCalled = true
		mode, ok := GetTxAuthMode(nextCtx)
		require.True(t, ok)
		require.Equal(t, TxAuthModeMina, mode)
		require.Equal(t, tx, nextTx)
		require.False(t, simulate)
		return nextCtx.WithLogger(log.NewNopLogger()), nil
	}

	_, err := decorator.AnteHandle(ctx, tx, false, next)

	require.NoError(t, err)
	require.True(t, nextCalled)
}
