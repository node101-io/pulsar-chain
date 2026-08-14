package ante

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log"
	txsigning "cosmossdk.io/x/tx/signing"
	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	signing "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/node101-io/mina-signer-go/publickey"
	"github.com/node101-io/mina-signer-go/signature"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
)

// MinaVerifier verifies tx signatures using Mina cryptography.
type MinaVerifier struct {
	keyregistryKeeper        *keyregistrykeeper.Keeper
	accountKeeper            authante.AccountKeeper
	signModeHandler          *txsigning.HandlerMap
	walletSignatureNetworkID mina.NetworkID
	logger                   log.Logger
}

// NewMinaVerifier creates a new Mina signature verifier.
func NewMinaVerifier(
	keyregistryKeeper *keyregistrykeeper.Keeper,
	accountKeeper authante.AccountKeeper,
	signModeHandler *txsigning.HandlerMap,
	walletSignatureNetworkID mina.NetworkID,
	logger log.Logger,
) MinaVerifier {
	return MinaVerifier{
		keyregistryKeeper:        keyregistryKeeper,
		accountKeeper:            accountKeeper,
		signModeHandler:          signModeHandler,
		walletSignatureNetworkID: walletSignatureNetworkID,
		logger:                   logger,
	}
}

// resolveAuroFieldSignatureNetworkID maps the configured Mina source network
// to the domain currently used by Auro signFields signatures.
func resolveAuroFieldSignatureNetworkID(networkID mina.NetworkID) (mina.NetworkID, error) {
	switch networkID {
	case mina.TestNet, mina.MainNet:
		return mina.TestNet, nil
	default:
		return "", fmt.Errorf("unsupported Mina network ID %q", networkID)
	}
}

// VerifySignatures verifies signatures for a Mina-authenticated tx.
func (v MinaVerifier) VerifySignatures(ctx sdk.Context, tx sdk.Tx, simulate bool) error {
	sigTx, ok := tx.(authsigning.Tx)
	if !ok {
		return errorsmod.Wrap(sdkerrors.ErrTxDecode, "invalid transaction type")
	}

	if unorderedTx, ok := tx.(sdk.TxWithUnordered); ok && unorderedTx.GetUnordered() {
		return errorsmod.Wrap(sdkerrors.ErrNotSupported, "unordered Mina transactions are not supported")
	}

	sigs, err := sigTx.GetSignaturesV2()
	if err != nil {
		return err
	}

	signers, err := sigTx.GetSigners()
	if err != nil {
		return err
	}

	if len(sigs) != len(signers) {
		return errorsmod.Wrapf(
			sdkerrors.ErrUnauthorized,
			"invalid number of signer;  expected: %d, got %d",
			len(signers),
			len(sigs),
		)
	}

	for i, sig := range sigs {
		acc, err := authante.GetSignerAcc(ctx, v.accountKeeper, signers[i])
		if err != nil {
			return err
		}

		if sig.Sequence != acc.GetSequence() {
			return errorsmod.Wrapf(
				sdkerrors.ErrWrongSequence,
				"account sequence mismatch, expected %d, got %d",
				acc.GetSequence(),
				sig.Sequence,
			)
		}

		if simulate || ctx.IsReCheckTx() || !ctx.IsSigverifyTx() {
			v.logger.Debug("skipping Mina signature verification", "simulate", simulate, "recheck", ctx.IsReCheckTx())
			continue
		}

		singleSig, ok := sig.Data.(*signing.SingleSignatureData)
		if !ok {
			return errorsmod.Wrap(
				sdkerrors.ErrInvalidType,
				"mina transactions require single signature data",
			)
		}

		if err := v.verifySingleSignature(ctx, tx, acc, singleSig, sig.Sequence); err != nil {
			return err
		}
	}

	return nil
}

func (v MinaVerifier) verifySingleSignature(
	ctx sdk.Context,
	tx sdk.Tx,
	account sdk.AccountI,
	signatureData *signing.SingleSignatureData,
	sequence uint64,
) error {
	// TODO: Support SIGN_MODE_DIRECT_AUX after defining its authorization
	// contract and adding wallet interoperability vectors.
	if signatureData.SignMode != signing.SignMode_SIGN_MODE_DIRECT {
		return errorsmod.Wrapf(
			sdkerrors.ErrNotSupported,
			"unsupported Mina transaction sign mode: %s",
			signatureData.SignMode,
		)
	}

	cosmosPubKey := account.GetPubKey()
	if cosmosPubKey == nil {
		return errorsmod.Wrapf(
			sdkerrors.ErrInvalidPubKey,
			"no Cosmos public key found for signer %s",
			account.GetAddress().String(),
		)
	}

	minaPubKeyBytes, err := v.keyregistryKeeper.UserGetCosmosToMina(ctx, cosmosPubKey.Bytes())
	if err != nil {
		return errorsmod.Wrapf(
			sdkerrors.ErrUnauthorized,
			"no Mina public key registered for signer %s",
			account.GetAddress().String(),
		)
	}

	minaPubKey, err := publickey.NewPublicKeyFromBytes(minaPubKeyBytes, v.walletSignatureNetworkID)
	if err != nil {
		return errorsmod.Wrapf(
			sdkerrors.ErrInvalidPubKey,
			"failed to parse Mina public key: %v",
			err,
		)
	}

	minaSignature, err := signature.NewSignatureFromBytes(signatureData.Signature)
	if err != nil {
		return errorsmod.Wrapf(
			sdkerrors.ErrInvalidType,
			"failed to parse Mina signature: %v",
			err,
		)
	}

	var accountNumber uint64
	if ctx.BlockHeight() > 0 {
		accountNumber = account.GetAccountNumber()
	}

	signBytes, err := authsigning.GetSignBytesAdapter(
		ctx,
		v.signModeHandler,
		signatureData.SignMode,
		authsigning.SignerData{
			Address:       account.GetAddress().String(),
			ChainID:       ctx.ChainID(),
			AccountNumber: accountNumber,
			Sequence:      sequence,
		},
		tx,
	)
	if err != nil {
		return errorsmod.Wrapf(
			sdkerrors.ErrInvalidType,
			"failed to generate sign bytes: %v",
			err,
		)
	}

	// Verified as a FIELD signature over the challenge derived from the sign
	// bytes, not as a byte signature over the bytes themselves: field signing
	// is the one scheme browser wallets can produce (Auro signFields), and the
	// challenge commits to the full sign bytes, so nothing is lost in the
	// reduction. This mirrors how x/keyregistry verifies wallet signatures on
	// registration — see BuildTxSigningChallenge for the derivation contract.
	challenge, err := BuildTxSigningChallenge(signBytes)
	if err != nil {
		return err
	}

	valid, err := minaPubKey.VerifyField(minaSignature, challenge)
	if err != nil {
		return err
	}

	if !valid {
		return errorsmod.Wrap(sdkerrors.ErrUnauthorized, "Mina signature verification failed")
	}

	return nil
}
