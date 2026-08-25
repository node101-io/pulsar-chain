package ante

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// MinaSignatureVerifier verifies Mina-authenticated tx signatures.
type MinaSignatureVerifier interface {
	VerifySignatures(ctx sdk.Context, tx sdk.Tx, simulate bool) error
}

// SmartAccountSignatureVerifier verifies smart-account-authenticated tx signatures.
type SmartAccountSignatureVerifier interface {
	VerifySignatures(ctx sdk.Context, tx sdk.Tx, simulate bool) error
}

// RoutedSigVerificationDecorator routes signature verification based on tx auth mode.
type RoutedSigVerificationDecorator struct {
	cosmosDecorator      sdk.AnteDecorator
	minaVerifier         MinaSignatureVerifier
	smartAccountVerifier SmartAccountSignatureVerifier
}

// NewRoutedSigVerificationDecorator creates a routed signature verification decorator.
func NewRoutedSigVerificationDecorator(
	cosmosDecorator sdk.AnteDecorator,
	minaVerifier MinaSignatureVerifier,
	smartAccountVerifier SmartAccountSignatureVerifier,
) RoutedSigVerificationDecorator {
	return RoutedSigVerificationDecorator{
		cosmosDecorator:      cosmosDecorator,
		minaVerifier:         minaVerifier,
		smartAccountVerifier: smartAccountVerifier,
	}
}

// AnteHandle implements sdk.AnteDecorator.
func (d RoutedSigVerificationDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	mode, err := getOrResolveTxAuthMode(ctx, tx)
	if err != nil {
		return ctx, err
	}

	switch mode {
	case TxAuthModeCosmos:
		return d.cosmosDecorator.AnteHandle(ctx, tx, simulate, next)
	case TxAuthModeMina:
		if err := d.minaVerifier.VerifySignatures(ctx, tx, simulate); err != nil {
			return ctx, err
		}

	case TxAuthModeSmartAccount:
		if err := d.smartAccountVerifier.VerifySignatures(ctx, tx, simulate); err != nil {
			return ctx, err
		}

	default:
		return ctx, errorsmod.Wrap(
			sdkerrors.ErrInvalidRequest,
			"unsupported transaction authentication mode",
		)
	}

	return next(ctx, tx, simulate)
}
