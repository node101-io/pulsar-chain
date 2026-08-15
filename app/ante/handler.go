package ante

import (
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	txsigning "cosmossdk.io/x/tx/signing"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	signing "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
)

// HandlerOptions are the options required for constructing the app ante handler.
type HandlerOptions struct {
	AccountKeeper          authante.AccountKeeper
	BankKeeper             authtypes.BankKeeper
	ExtensionOptionChecker authante.ExtensionOptionChecker
	FeegrantKeeper         authante.FeegrantKeeper
	SignModeHandler        *txsigning.HandlerMap
	SigGasConsumer         func(meter storetypes.GasMeter, sig signing.SignatureV2, params authtypes.Params) error
	TxFeeChecker           authante.TxFeeChecker
	SigVerifyOptions       []authante.SigVerificationDecoratorOption
	KeyregistryKeeper      *keyregistrykeeper.Keeper
	MinaNetworkID          string
	Logger                 log.Logger
}

// NewAnteHandler returns the chain ante handler.
func NewAnteHandler(options HandlerOptions) (sdk.AnteHandler, error) {
	if options.AccountKeeper == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "account keeper is required for ante builder")
	}

	if options.BankKeeper == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "bank keeper is required for ante builder")
	}

	if options.SignModeHandler == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "sign mode handler is required for ante builder")
	}

	if options.KeyregistryKeeper == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "keyregistry keeper is required for ante builder")
	}

	if options.MinaNetworkID == "" {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "mina network ID is required for ante builder")
	}
	walletSignatureNetworkID, err := resolveAuroFieldSignatureNetworkID(mina.NetworkID(options.MinaNetworkID))
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrLogic, "resolve Auro field signature domain: %v", err)
	}

	if options.Logger == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "logger is required for ante builder")
	}

	cosmosSetPubKey := authante.NewSetPubKeyDecorator(options.AccountKeeper)
	cosmosValidateSigCount := authante.NewValidateSigCountDecorator(options.AccountKeeper)
	cosmosSigGasConsume := authante.NewSigGasConsumeDecorator(options.AccountKeeper, options.SigGasConsumer)
	cosmosSigVerify := authante.NewSigVerificationDecorator(
		options.AccountKeeper,
		options.SignModeHandler,
		options.SigVerifyOptions...,
	)

	minaVerifier := NewMinaVerifier(
		options.KeyregistryKeeper,
		options.AccountKeeper,
		options.SignModeHandler,
		walletSignatureNetworkID,
		options.Logger,
	)

	anteDecorators := []sdk.AnteDecorator{
		authante.NewSetUpContextDecorator(),
		authante.NewExtensionOptionsDecorator(NewTxAuthExtensionOptionChecker(options.ExtensionOptionChecker)),
		authante.NewValidateBasicDecorator(),
		authante.NewTxTimeoutHeightDecorator(),
		authante.NewValidateMemoDecorator(options.AccountKeeper),
		authante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper),
		authante.NewDeductFeeDecorator(options.AccountKeeper, options.BankKeeper, options.FeegrantKeeper, options.TxFeeChecker),
		NewTxAuthModeDecorator(),
		NewRoutedSetPubKeyDecorator(cosmosSetPubKey),
		NewRoutedValidateSigCountDecorator(options.AccountKeeper, cosmosValidateSigCount),
		NewRoutedSigGasConsumeDecorator(options.AccountKeeper, cosmosSigGasConsume),
		NewRoutedSigVerificationDecorator(cosmosSigVerify, minaVerifier),
		NewStakingDecorator(options.KeyregistryKeeper),
		authante.NewIncrementSequenceDecorator(options.AccountKeeper),
	}

	return sdk.ChainAnteDecorators(anteDecorators...), nil
}
