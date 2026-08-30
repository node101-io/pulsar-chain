package ante

import (
	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	signing "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
)

// RoutedValidateSigCountDecorator routes signature-count validation by tx auth mode.
type RoutedValidateSigCountDecorator struct {
	accountKeeper   authante.AccountKeeper
	cosmosDecorator sdk.AnteDecorator
}

// NewRoutedValidateSigCountDecorator creates a routed signature-count decorator.
func NewRoutedValidateSigCountDecorator(
	accountKeeper authante.AccountKeeper,
	cosmosDecorator sdk.AnteDecorator,
) RoutedValidateSigCountDecorator {
	return RoutedValidateSigCountDecorator{
		accountKeeper:   accountKeeper,
		cosmosDecorator: cosmosDecorator,
	}
}

// AnteHandle implements sdk.AnteDecorator.
func (d RoutedValidateSigCountDecorator) AnteHandle(
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
		return ctx, errorsmod.Wrap(sdkerrors.ErrTxDecode, "Tx must be a sigTx")
	}

	params := d.accountKeeper.GetParams(ctx)
	sigs, err := sigTx.GetSignaturesV2()
	if err != nil {
		return ctx, err
	}

	if mode == TxAuthModeSmartAccount {
		if len(sigs) != 1 {
			return ctx, errorsmod.Wrapf(
				sdkerrors.ErrUnauthorized,
				"smart-account transactions require exactly one signature: got %d",
				len(sigs),
			)
		}

		switch sigs[0].Data.(type) {
		case *signing.SingleSignatureData:
			return next(ctx, tx, simulate)

		case *signing.MultiSignatureData:
			return ctx, errorsmod.Wrap(
				sdkerrors.ErrInvalidType,
				"smart-account transactions do not support multisig signatures",
			)

		default:
			return ctx, errorsmod.Wrapf(
				sdkerrors.ErrInvalidType,
				"unexpected signature data type %T",
				sigs[0].Data,
			)
		}
	}

	sigCount := 0
	for _, sig := range sigs {
		switch sig.Data.(type) {
		case *signing.SingleSignatureData:
			sigCount++
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

		if uint64(sigCount) > params.TxSigLimit {
			return ctx, errorsmod.Wrapf(
				sdkerrors.ErrTooManySignatures,
				"signatures: %d, limit: %d",
				sigCount,
				params.TxSigLimit,
			)
		}
	}

	return next(ctx, tx, simulate)
}
