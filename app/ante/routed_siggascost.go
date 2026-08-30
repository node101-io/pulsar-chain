package ante

import (
	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	signing "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
)

// RoutedSigGasConsumeDecorator routes signature gas accounting by tx auth mode.
type RoutedSigGasConsumeDecorator struct {
	accountKeeper   authante.AccountKeeper
	cosmosDecorator sdk.AnteDecorator
}

// NewRoutedSigGasConsumeDecorator creates a routed signature gas decorator.
func NewRoutedSigGasConsumeDecorator(
	accountKeeper authante.AccountKeeper,
	cosmosDecorator sdk.AnteDecorator,
) RoutedSigGasConsumeDecorator {
	return RoutedSigGasConsumeDecorator{
		accountKeeper:   accountKeeper,
		cosmosDecorator: cosmosDecorator,
	}
}

// AnteHandle implements sdk.AnteDecorator.
func (d RoutedSigGasConsumeDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	mode, err := getOrResolveTxAuthMode(ctx, tx)
	if err != nil {
		return ctx, err
	}

	if mode == TxAuthModeCosmos {
		return d.cosmosDecorator.AnteHandle(ctx, tx, simulate, next)
	}

	sigTx, ok := tx.(authsigning.SigVerifiableTx)
	if !ok {
		return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "invalid transaction type")
	}

	params := d.accountKeeper.GetParams(ctx)
	sigs, err := sigTx.GetSignaturesV2()
	if err != nil {
		return ctx, err
	}

	if mode == TxAuthModeSmartAccount {
		for _, sig := range sigs {
			switch sig.Data.(type) {
			case *signing.SingleSignatureData:
				ctx.GasMeter().ConsumeGas(
					params.SigVerifyCostED25519,
					"ante verify: smart account",
				)

			case *signing.MultiSignatureData:
				return ctx, errorsmod.Wrap(
					sdkerrors.ErrInvalidType,
					"smart-account transactions do not support multisig signatures",
				)

			default:
				return ctx, errorsmod.Wrapf(
					sdkerrors.ErrInvalidType,
					"unexpected signature data type %T",
					sig.Data,
				)
			}
		}

		return next(ctx, tx, simulate)
	}

	for _, sig := range sigs {
		switch sig.Data.(type) {
		case *signing.SingleSignatureData:
			ctx.GasMeter().ConsumeGas(params.SigVerifyCostSecp256k1, "ante verify: mina")
		case *signing.MultiSignatureData:
			return ctx, errorsmod.Wrap(
				sdkerrors.ErrInvalidType,
				"mina transactions do not support multisig signatures",
			)
		default:
			return ctx, errorsmod.Wrapf(
				sdkerrors.ErrInvalidType,
				"unexpected signature data type %T",
				sig.Data,
			)
		}
	}

	return next(ctx, tx, simulate)
}
