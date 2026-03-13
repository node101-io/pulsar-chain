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

func mustTxAuthModeExtensionAny(t *testing.T, mode antetypes.TxAuthMode) *codectypes.Any {
	t.Helper()

	bz, err := gogoproto.Marshal(&antetypes.TxAuthModeExtension{TxAuthMode: mode})
	require.NoError(t, err)

	return &codectypes.Any{
		TypeUrl: txAuthModeExtensionTypeURL,
		Value:   bz,
	}
}

func TestResolveTxAuthModeDefaultsToCosmos(t *testing.T) {
	t.Parallel()

	mode, err := ResolveTxAuthMode(stubExtensionTx{})

	require.NoError(t, err)
	require.Equal(t, TxAuthModeCosmos, mode)
}

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
