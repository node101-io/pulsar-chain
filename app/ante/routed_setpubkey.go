package ante

import sdk "github.com/cosmos/cosmos-sdk/types"

// RoutedSetPubKeyDecorator routes pubkey handling based on the resolved tx auth mode.
type RoutedSetPubKeyDecorator struct {
	cosmosDecorator sdk.AnteDecorator
}

// NewRoutedSetPubKeyDecorator creates a new routed set-pubkey decorator.
func NewRoutedSetPubKeyDecorator(cosmosDecorator sdk.AnteDecorator) RoutedSetPubKeyDecorator {
	return RoutedSetPubKeyDecorator{cosmosDecorator: cosmosDecorator}
}

// AnteHandle implements sdk.AnteDecorator.
func (d RoutedSetPubKeyDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	mode, err := getOrResolveTxAuthMode(ctx, tx)
	if err != nil {
		return ctx, err
	}

	if mode == TxAuthModeMina || mode == TxAuthModeSmartAccount {
		return next(ctx, tx, simulate)
	}

	return d.cosmosDecorator.AnteHandle(ctx, tx, simulate, next)
}
