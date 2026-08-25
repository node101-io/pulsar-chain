package ante

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	txsigning "cosmossdk.io/x/tx/signing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	signing "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	gogoproto "github.com/cosmos/gogoproto/proto"

	antetypes "github.com/node101-io/pulsar-chain/app/ante/types"
	smartaccountstypes "github.com/node101-io/pulsar-chain/x/smartaccounts/types"
)

type SmartAccountKeeper interface {
	IsSessionKeyAuthorized(ctx context.Context, identity, accountAddr, publicKey []byte) (bool, error)
}

// SmartAccountVerifier verifies smart-account transaction signatures.
type SmartAccountVerifier struct {
	smartAccountsKeeper SmartAccountKeeper
	accountKeeper       authante.AccountKeeper
	signModeHandler     *txsigning.HandlerMap
}

// NewSmartAccountVerifier creates a smart-account signature verifier.
func NewSmartAccountVerifier(
	smartAccountsKeeper SmartAccountKeeper,
	accountKeeper authante.AccountKeeper,
	signModeHandler *txsigning.HandlerMap,
) SmartAccountVerifier {
	return SmartAccountVerifier{
		smartAccountsKeeper: smartAccountsKeeper,
		accountKeeper:       accountKeeper,
		signModeHandler:     signModeHandler,
	}
}

// VerifySignatures verifies a smart-account transaction.
func (v SmartAccountVerifier) VerifySignatures(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
) error {
	sigTx, ok := tx.(authsigning.Tx)
	if !ok {
		return errorsmod.Wrap(
			sdkerrors.ErrTxDecode,
			"invalid smart-account transaction type",
		)
	}

	if sigTx.GetUnordered() {
		return errorsmod.Wrap(
			sdkerrors.ErrNotSupported,
			"unordered smart-account transactions are not supported",
		)
	}

	signatures, err := sigTx.GetSignaturesV2()
	if err != nil {
		return err
	}

	signers, err := sigTx.GetSigners()
	if err != nil {
		return err
	}

	if len(signatures) != 1 || len(signers) != 1 {
		return errorsmod.Wrapf(
			sdkerrors.ErrUnauthorized,
			"smart-account transactions require one signer and one signature: signers %d, signatures %d",
			len(signers),
			len(signatures),
		)
	}

	signature := signatures[0]

	sessionPublicKey, ok := signature.PubKey.(*ed25519.PubKey)
	if !ok {
		return errorsmod.Wrap(
			sdkerrors.ErrInvalidPubKey,
			"smart-account transactions require an Ed25519 session public key",
		)
	}

	signatureData, ok := signature.Data.(*signing.SingleSignatureData)
	if !ok {
		return errorsmod.Wrap(
			sdkerrors.ErrInvalidType,
			"smart-account transactions require single signature data",
		)
	}

	if signatureData.SignMode != signing.SignMode_SIGN_MODE_DIRECT {
		return errorsmod.Wrapf(
			sdkerrors.ErrNotSupported,
			"unsupported smart-account sign mode: %s",
			signatureData.SignMode,
		)
	}

	identity, err := smartAccountIdentityFromTx(tx)
	if err != nil {
		return err
	}

	accountAddress := signers[0]

	authorized, err := v.smartAccountsKeeper.IsSessionKeyAuthorized(
		ctx,
		identity,
		accountAddress,
		sessionPublicKey.Bytes(),
	)
	if err != nil {
		return err
	}
	if !authorized {
		return errorsmod.Wrap(
			sdkerrors.ErrUnauthorized,
			"session public key is not authorized for smart account",
		)
	}

	account, err := authante.GetSignerAcc(
		ctx,
		v.accountKeeper,
		accountAddress,
	)
	if err != nil {
		return err
	}

	if signature.Sequence != account.GetSequence() {
		return errorsmod.Wrapf(
			sdkerrors.ErrWrongSequence,
			"account sequence mismatch, expected %d, got %d",
			account.GetSequence(),
			signature.Sequence,
		)
	}

	if simulate || ctx.IsReCheckTx() || !ctx.IsSigverifyTx() {
		return nil
	}

	accountNumber := account.GetAccountNumber()

	signBytes, err := authsigning.GetSignBytesAdapter(
		ctx,
		v.signModeHandler,
		signatureData.SignMode,
		authsigning.SignerData{
			Address:       account.GetAddress().String(),
			ChainID:       ctx.ChainID(),
			AccountNumber: accountNumber,
			Sequence:      signature.Sequence,
		},
		tx,
	)
	if err != nil {
		return errorsmod.Wrapf(
			sdkerrors.ErrInvalidType,
			"generate smart-account sign bytes: %v",
			err,
		)
	}

	if !sessionPublicKey.VerifySignature(
		signBytes,
		signatureData.Signature,
	) {
		return errorsmod.Wrap(
			sdkerrors.ErrUnauthorized,
			"smart-account signature verification failed",
		)
	}

	return nil
}

func smartAccountIdentityFromTx(tx sdk.Tx) ([]byte, error) {
	txWithExtensions, ok := tx.(HasExtensionOptionsTx)
	if !ok {
		return nil, errorsmod.Wrap(
			sdkerrors.ErrInvalidRequest,
			"smart-account transaction has no extension options",
		)
	}

	var identity []byte
	found := false

	for _, option := range txWithExtensions.GetExtensionOptions() {
		if option == nil || option.TypeUrl != txAuthModeExtensionTypeURL {
			continue
		}

		if found {
			return nil, errorsmod.Wrap(
				sdkerrors.ErrInvalidRequest,
				"multiple tx auth mode extensions found",
			)
		}
		found = true

		extension := &antetypes.TxAuthModeExtension{}
		if err := gogoproto.Unmarshal(option.Value, extension); err != nil {
			return nil, errorsmod.Wrapf(
				sdkerrors.ErrTxDecode,
				"invalid tx auth mode extension: %v",
				err,
			)
		}

		if extension.TxAuthMode != antetypes.TX_AUTH_MODE_SMART_ACCOUNT {
			return nil, errorsmod.Wrap(
				sdkerrors.ErrInvalidRequest,
				"smart-account verifier received a non-smart-account extension",
			)
		}

		identity = extension.SmartAccountIdentity
	}

	if !found {
		return nil, errorsmod.Wrap(
			sdkerrors.ErrInvalidRequest,
			"smart-account auth mode extension not found",
		)
	}

	if len(identity) != smartaccountstypes.IdentitySize {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"smart-account identity must be %d bytes: got %d",
			smartaccountstypes.IdentitySize,
			len(identity),
		)
	}

	return identity, nil
}
