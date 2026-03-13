package ante

import sdk "github.com/cosmos/cosmos-sdk/types"

// MinaSignatureVerifier verifies Mina-authenticated tx signatures.
type MinaSignatureVerifier interface {
	VerifySignatures(ctx sdk.Context, tx sdk.Tx, simulate bool) error
}

// RoutedSigVerificationDecorator routes signature verification based on tx auth mode.
type RoutedSigVerificationDecorator struct {
	cosmosDecorator sdk.AnteDecorator
	minaVerifier    MinaSignatureVerifier
}

// NewRoutedSigVerificationDecorator creates a routed signature verification decorator.
func NewRoutedSigVerificationDecorator(
	cosmosDecorator sdk.AnteDecorator,
	minaVerifier MinaSignatureVerifier,
) RoutedSigVerificationDecorator {
	return RoutedSigVerificationDecorator{
		cosmosDecorator: cosmosDecorator,
		minaVerifier:    minaVerifier,
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

	if mode != TxAuthModeMina {
		return d.cosmosDecorator.AnteHandle(ctx, tx, simulate, next)
	}

	if err := d.minaVerifier.VerifySignatures(ctx, tx, simulate); err != nil {
		return ctx, err
	}

	return next(ctx, tx, simulate)
}
