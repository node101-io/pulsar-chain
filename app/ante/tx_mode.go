package ante

import (
	errorsmod "cosmossdk.io/errors"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	gogoproto "github.com/cosmos/gogoproto/proto"

	antetypes "github.com/node101-io/pulsar-chain/app/ante/types"
)

const txAuthModeExtensionTypeURL = "/pulsarchain.ante.v1.TxAuthModeExtension"

type txAuthModeContextKey struct{}

// TxAuthMode is the auth verification mode selected for a tx.
type TxAuthMode uint8

const (
	TxAuthModeCosmos TxAuthMode = iota
	TxAuthModeMina
)

// RegisterInterfaces registers ante extension interfaces.
func RegisterInterfaces(registrar codectypes.InterfaceRegistry) {
	antetypes.RegisterInterfaces(registrar)
}

// HasExtensionOptionsTx is the subset of tx functionality needed for auth mode resolution.
type HasExtensionOptionsTx interface {
	GetExtensionOptions() []*codectypes.Any
}

// NewTxAuthExtensionOptionChecker returns an extension checker that accepts the tx auth mode extension.
func NewTxAuthExtensionOptionChecker(additional authante.ExtensionOptionChecker) authante.ExtensionOptionChecker {
	return func(extOption *codectypes.Any) bool {
		if extOption != nil && extOption.TypeUrl == txAuthModeExtensionTypeURL {
			return true
		}

		if additional == nil {
			return false
		}

		return additional(extOption)
	}
}

// NewTxAuthModeDecorator resolves the tx auth mode once and stores it in context.
func NewTxAuthModeDecorator() TxAuthModeDecorator {
	return TxAuthModeDecorator{}
}

// TxAuthModeDecorator stores the resolved tx auth mode in context for downstream decorators.
type TxAuthModeDecorator struct{}

// AnteHandle implements sdk.AnteDecorator.
func (d TxAuthModeDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	mode, err := ResolveTxAuthMode(tx)
	if err != nil {
		return ctx, err
	}

	return next(setTxAuthMode(ctx, mode), tx, simulate)
}

// ResolveTxAuthMode resolves the tx auth mode from extension options.
func ResolveTxAuthMode(tx sdk.Tx) (TxAuthMode, error) {
	txWithExtensions, ok := tx.(HasExtensionOptionsTx)
	if !ok {
		return TxAuthModeCosmos, nil
	}

	var found *antetypes.TxAuthModeExtension
	for _, extOption := range txWithExtensions.GetExtensionOptions() {
		if extOption == nil || extOption.TypeUrl != txAuthModeExtensionTypeURL {
			continue
		}

		if found != nil {
			return TxAuthModeCosmos, errorsmod.Wrap(
				sdkerrors.ErrInvalidRequest,
				"multiple tx auth mode extensions found",
			)
		}

		found = &antetypes.TxAuthModeExtension{}
		if err := gogoproto.Unmarshal(extOption.Value, found); err != nil {
			return TxAuthModeCosmos, errorsmod.Wrapf(
				sdkerrors.ErrTxDecode,
				"invalid tx auth mode extension: %v",
				err,
			)
		}
	}

	if found == nil {
		return TxAuthModeCosmos, nil
	}

	switch found.TxAuthMode {
	case antetypes.TX_AUTH_MODE_COSMOS:
		return TxAuthModeCosmos, nil
	case antetypes.TX_AUTH_MODE_MINA:
		return TxAuthModeMina, nil
	default:
		return TxAuthModeCosmos, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"unsupported tx auth mode: %s",
			found.TxAuthMode.String(),
		)
	}
}

func setTxAuthMode(ctx sdk.Context, mode TxAuthMode) sdk.Context {
	return ctx.WithValue(txAuthModeContextKey{}, mode)
}

// GetTxAuthMode returns the tx auth mode from context if already resolved.
func GetTxAuthMode(ctx sdk.Context) (TxAuthMode, bool) {
	mode, ok := ctx.Value(txAuthModeContextKey{}).(TxAuthMode)
	return mode, ok
}

func getOrResolveTxAuthMode(ctx sdk.Context, tx sdk.Tx) (TxAuthMode, error) {
	if mode, ok := GetTxAuthMode(ctx); ok {
		return mode, nil
	}

	return ResolveTxAuthMode(tx)
}
