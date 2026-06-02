package ante

import (
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

// DefaultMinaNetworkID is the default Mina network used by the chain.
const DefaultMinaNetworkID = "devnet"

// MinaVerifier verifies tx signatures using Mina cryptography.
type MinaVerifier struct {
	keyregistryKeeper *keyregistrykeeper.Keeper
	accountKeeper     authante.AccountKeeper
	signModeHandler   *txsigning.HandlerMap
	networkID         string
	logger            log.Logger
}

// NewMinaVerifier creates a new Mina signature verifier.
func NewMinaVerifier(
	keyregistryKeeper *keyregistrykeeper.Keeper,
	accountKeeper authante.AccountKeeper,
	signModeHandler *txsigning.HandlerMap,
	networkID string,
	logger log.Logger,
) MinaVerifier {
	return MinaVerifier{
		keyregistryKeeper: keyregistryKeeper,
		accountKeeper:     accountKeeper,
		signModeHandler:   signModeHandler,
		networkID:         networkID,
		logger:            logger,
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

	minaPubKey, err := publickey.NewPublicKeyFromBytes(minaPubKeyBytes, mina.NetworkID(v.networkID))
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

	valid, err := minaPubKey.VerifyString(minaSignature, string(signBytes))
	if err != nil {
		return err
	}

	if !valid {
		return errorsmod.Wrap(sdkerrors.ErrUnauthorized, "Mina signature verification failed")
	}

	return nil
}
