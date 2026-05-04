package ante

import (
	"context"
	"errors"
	"testing"
	"time"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	txsigning "cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/node101-io/mina-signer-go/keys"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	keyregistrymodule "github.com/node101-io/pulsar-chain/x/keyregistry/module"
	keyregistrytypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"
)

type verifierAccountKeeper struct {
	account sdk.AccountI
	params  authtypes.Params
}

func (k verifierAccountKeeper) GetParams(context.Context) authtypes.Params {
	if k.params.TxSigLimit == 0 {
		return authtypes.DefaultParams()
	}

	return k.params
}

func (k verifierAccountKeeper) GetAccount(context.Context, sdk.AccAddress) sdk.AccountI {
	return k.account
}

func (k verifierAccountKeeper) SetAccount(context.Context, sdk.AccountI) {}

func (k verifierAccountKeeper) GetModuleAddress(string) sdk.AccAddress { return nil }

func (k verifierAccountKeeper) AddressCodec() address.Codec { return nil }

func (k verifierAccountKeeper) UnorderedTransactionsEnabled() bool { return false }

func (k verifierAccountKeeper) RemoveExpiredUnorderedNonces(sdk.Context) error { return nil }

func (k verifierAccountKeeper) TryAddUnorderedNonce(sdk.Context, []byte, time.Time) error {
	return nil
}

type stubAuthTx struct {
	signers    [][]byte
	signersErr error
	sigs       []signingtypes.SignatureV2
	sigsErr    error
	unordered  bool
}

func (tx stubAuthTx) GetMsgs() []sdk.Msg { return nil }

func (tx stubAuthTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func (tx stubAuthTx) GetSigners() ([][]byte, error) {
	if tx.signersErr != nil {
		return nil, tx.signersErr
	}

	return tx.signers, nil
}

func (tx stubAuthTx) GetPubKeys() ([]cryptotypes.PubKey, error) { return nil, nil }

func (tx stubAuthTx) GetSignaturesV2() ([]signingtypes.SignatureV2, error) {
	if tx.sigsErr != nil {
		return nil, tx.sigsErr
	}

	return tx.sigs, nil
}

func (tx stubAuthTx) GetMemo() string { return "" }

func (tx stubAuthTx) GetTimeoutHeight() uint64 { return 0 }

func (tx stubAuthTx) GetTimeoutTimeStamp() time.Time { return time.Time{} }

func (tx stubAuthTx) GetUnordered() bool { return tx.unordered }

func (tx stubAuthTx) ValidateBasic() error { return nil }

func (tx stubAuthTx) GetGas() uint64 { return 0 }

func (tx stubAuthTx) GetFee() sdk.Coins { return nil }

func (tx stubAuthTx) FeePayer() []byte { return nil }

func (tx stubAuthTx) FeeGranter() []byte { return nil }

func newKeyregistryKeeperForTest(t *testing.T) (sdk.Context, *keyregistrykeeper.Keeper) {
	t.Helper()

	encCfg := moduletestutil.MakeTestEncodingConfig(keyregistrymodule.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(keyregistrytypes.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(keyregistrytypes.GovModuleName)

	keeper := keyregistrykeeper.NewKeeper(
		storeService,
		encCfg.Codec,
		addressCodec,
		authority,
	)

	err := keeper.InitGenesis(ctx, *keyregistrytypes.DefaultGenesis())
	require.NoError(t, err)

	return ctx, &keeper
}

func registerMinaPublicKeyForAccount(
	t *testing.T,
	ctx sdk.Context,
	keeper *keyregistrykeeper.Keeper,
	account sdk.AccountI,
	minaPubKey []byte,
) {
	t.Helper()

	cosmosPubKey := account.GetPubKey()
	require.NotNil(t, cosmosPubKey)

	err := keeper.InitGenesis(ctx, keyregistrytypes.GenesisState{
		Params: keyregistrytypes.DefaultParams(),
		UserKeyPairs: []*keyregistrytypes.UserPublicKeyPair{
			{
				CosmosKey: cosmosPubKey.Bytes(),
				MinaKey:   minaPubKey,
			},
		},
		ValidatorKeyPairs: keyregistrytypes.DefaultValidatorPublicKeyPair(),
	})
	require.NoError(t, err)
}

// newVerifierForTest keeps the early-branch tests lightweight by wiring a minimal verifier.
// It is paired with an empty HandlerMap because those tests never build real sign bytes.
func newVerifierForTest(
	account sdk.AccountI,
	keyregistryKeeper *keyregistrykeeper.Keeper,
) MinaVerifier {
	return NewMinaVerifier(
		keyregistryKeeper,
		verifierAccountKeeper{account: account},
		&txsigning.HandlerMap{},
		DefaultMinaNetworkID,
		log.NewNopLogger(),
	)
}

type verifierEncodingConfig struct {
	TxConfig client.TxConfig
}

// newVerifierEncodingConfig builds a real tx config with address codecs enabled.
// The end-to-end Mina signature tests need this so signers and sign bytes match production encoding.
func newVerifierEncodingConfig(t *testing.T) verifierEncodingConfig {
	t.Helper()

	interfaceRegistry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: gogoproto.HybridResolver,
		SigningOptions: txsigning.Options{
			AddressCodec:          authcodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
			ValidatorAddressCodec: authcodec.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()),
		},
	})
	require.NoError(t, err)
	std.RegisterInterfaces(interfaceRegistry)
	auth.AppModuleBasic{}.RegisterInterfaces(interfaceRegistry)
	banktypes.RegisterInterfaces(interfaceRegistry)

	protoCodec := codec.NewProtoCodec(interfaceRegistry)
	txConfig := authtx.NewTxConfig(protoCodec, authtx.DefaultSignModes)

	return verifierEncodingConfig{TxConfig: txConfig}
}

// newVerifierWithRealSignModeHandlerForTest wires the verifier to a real sign-mode handler.
// The lower verifier branches call GetSignBytesAdapter, so stub handlers are not sufficient there.
func newVerifierWithRealSignModeHandlerForTest(
	t *testing.T,
	account sdk.AccountI,
	keyregistryKeeper *keyregistrykeeper.Keeper,
) (MinaVerifier, verifierEncodingConfig) {
	t.Helper()

	encoding := newVerifierEncodingConfig(t)

	return NewMinaVerifier(
		keyregistryKeeper,
		verifierAccountKeeper{account: account},
		encoding.TxConfig.SignModeHandler(),
		DefaultMinaNetworkID,
		log.NewNopLogger(),
	), encoding
}

// newVerifierAccount creates a signer account with real secp256k1 key material and sequence metadata.
// The verifier reads address, account number and sequence from this account during validation.
func newVerifierAccount(t *testing.T, accountNumber uint64, sequence uint64) (*secp256k1.PrivKey, sdk.AccountI) {
	t.Helper()

	cosmosPrivKey := secp256k1.GenPrivKey()
	address := sdk.AccAddress(cosmosPrivKey.PubKey().Address())

	return cosmosPrivKey, authtypes.NewBaseAccount(address, cosmosPrivKey.PubKey(), accountNumber, sequence)
}

// newMinaPrivateKeyForTest derives deterministic Mina keys from a tiny seed marker.
// Deterministic keys keep the signature tests reproducible while staying easy to read.
func newMinaPrivateKeyForTest(t *testing.T, marker byte) *keys.PrivateKey {
	t.Helper()

	var seed [32]byte
	seed[0] = marker

	privateKey := keys.NewPrivateKeyFromBytes(seed)
	require.NotNil(t, privateKey)

	return &privateKey
}

func minaPublicKeyBytesForTest(t *testing.T, privateKey *keys.PrivateKey) []byte {
	t.Helper()

	publicKey := privateKey.ToPublicKey()
	publicKeyBytes, err := publicKey.MarshalBytes()
	require.NoError(t, err)

	return publicKeyBytes
}

// buildVerifierTestTx creates the smallest real SDK tx shape the verifier can inspect and re-sign.
// Using a real TxBuilder keeps signer extraction and sign-byte generation aligned with production.
func buildVerifierTestTx(
	t *testing.T,
	txConfig client.TxConfig,
	from sdk.AccAddress,
	pubKey cryptotypes.PubKey,
	sequence uint64,
	signMode signingtypes.SignMode,
	signature []byte,
) sdk.Tx {
	t.Helper()

	builder := txConfig.NewTxBuilder()
	to := sdk.AccAddress([]byte("verifier-recipient-1"))

	err := builder.SetMsgs(banktypes.NewMsgSend(from, to, sdk.NewCoins(sdk.NewInt64Coin("pmina", 1))))
	require.NoError(t, err)

	builder.SetGasLimit(200000)
	builder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("pmina", 1)))
	builder.SetMemo("mina-verifier-test")

	err = builder.SetSignatures(signingtypes.SignatureV2{
		PubKey: pubKey,
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signMode,
			Signature: signature,
		},
		Sequence: sequence,
	})
	require.NoError(t, err)

	return builder.GetTx()
}

// buildVerifierSignBytes reproduces the exact sign-byte payload verifySingleSignature expects.
// Tests use it to create both valid and intentionally invalid Mina signatures against a real tx.
func buildVerifierSignBytes(
	t *testing.T,
	ctx sdk.Context,
	txConfig client.TxConfig,
	tx sdk.Tx,
	account sdk.AccountI,
	sequence uint64,
	signMode signingtypes.SignMode,
) []byte {
	t.Helper()

	var accountNumber uint64
	if ctx.BlockHeight() > 0 {
		accountNumber = account.GetAccountNumber()
	}

	signBytes, err := authsigning.GetSignBytesAdapter(
		ctx,
		txConfig.SignModeHandler(),
		signMode,
		authsigning.SignerData{
			Address:       account.GetAddress().String(),
			ChainID:       ctx.ChainID(),
			AccountNumber: accountNumber,
			Sequence:      sequence,
		},
		tx,
	)
	require.NoError(t, err)

	return signBytes
}

// signMinaBytes turns raw sign bytes into the wire-format payload consumed by the verifier.
// This keeps the success and crypto-failure tests working with real Mina signatures instead of stubs.
func signMinaBytes(
	t *testing.T,
	privateKey *keys.PrivateKey,
	message []byte,
	networkID string,
) []byte {
	t.Helper()

	signature, err := privateKey.SignMessage(string(message), networkID)
	require.NoError(t, err)

	signatureBytes, err := signature.MarshalBytes()
	require.NoError(t, err)

	return signatureBytes
}

// Unordered txs are intentionally unsupported on the Mina path.
// This guards an early policy branch before any signer or signature processing happens.
func TestMinaVerifierRejectsUnorderedTransactions(t *testing.T) {
	t.Parallel()

	address := sdk.AccAddress([]byte("verifier-address-001"))
	account := authtypes.NewBaseAccount(address, nil, 1, 7)
	verifier := newVerifierForTest(account, nil)

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), stubAuthTx{
		unordered: true,
	}, false)

	require.ErrorContains(t, err, "unordered Mina transactions are not supported")
}

// VerifySignatures only supports authsigning.Tx implementations.
// A plain sdk.Tx should fail before any signer inspection starts.
func TestMinaVerifierRejectsInvalidTransactionType(t *testing.T) {
	t.Parallel()
	_, account := newVerifierAccount(t, 1, 7)
	verifier := newVerifierForTest(account, nil)

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), stubBasicTx{}, false)

	require.ErrorIs(t, err, sdkerrors.ErrTxDecode)
	require.ErrorContains(t, err, "invalid transaction type")
}

// Signature loading errors come from the tx and should bubble up unchanged.
// The verifier should stop before touching account lookup or registry resolution.
func TestMinaVerifierPropagatesGetSignaturesError(t *testing.T) {
	t.Parallel()
	_, account := newVerifierAccount(t, 1, 7)
	verifier := newVerifierForTest(account, nil)
	expectedErr := errors.New("signatures unavailable")

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), stubAuthTx{
		sigsErr: expectedErr,
	}, false)

	require.ErrorIs(t, err, expectedErr)
}

// Signer loading errors are also tx-owned and should not be wrapped away.
// This covers the second early-return branch before signer/signature count validation.
func TestMinaVerifierPropagatesGetSignersError(t *testing.T) {
	t.Parallel()
	_, account := newVerifierAccount(t, 1, 7)
	verifier := newVerifierForTest(account, nil)
	expectedErr := errors.New("signers unavailable")

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), stubAuthTx{
		sigs:       []signingtypes.SignatureV2{{}},
		signersErr: expectedErr,
	}, false)

	require.ErrorIs(t, err, expectedErr)
}

// Each signer must have exactly one matching SignatureV2 entry.
// Mismatched lengths should fail before any account lookup or crypto work begins.
func TestMinaVerifierRejectsSignerSignatureCountMismatch(t *testing.T) {
	t.Parallel()
	address := sdk.AccAddress([]byte("verifier-address-mismatch"))
	_, account := newVerifierAccount(t, 1, 7)
	verifier := newVerifierForTest(account, nil)

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), stubAuthTx{
		signers: [][]byte{address},
		sigs:    nil,
	}, false)

	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.ErrorContains(t, err, "invalid number of signer")
}

// Sequence mismatches must fail before any expensive Mina-specific verification happens.
// This mirrors the standard auth invariant while still exercising the Mina verifier path.
func TestMinaVerifierRejectsWrongSequence(t *testing.T) {
	t.Parallel()

	address := sdk.AccAddress([]byte("verifier-address-002"))
	account := authtypes.NewBaseAccount(address, nil, 1, 7)
	verifier := newVerifierForTest(account, nil)

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), stubAuthTx{
		signers: [][]byte{address},
		sigs: []signingtypes.SignatureV2{
			{
				Sequence: 8,
				Data: &signingtypes.SingleSignatureData{
					SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
					Signature: []byte{1},
				},
			},
		},
	}, false)

	require.ErrorContains(t, err, "account sequence mismatch")
}

// Mina-auth only supports single signatures in this chain.
// Multisig payloads should be rejected before registry lookup or sign-byte generation.
func TestMinaVerifierRejectsNonSingleSignatureData(t *testing.T) {
	t.Parallel()

	address := sdk.AccAddress([]byte("verifier-address-003"))
	account := authtypes.NewBaseAccount(address, nil, 1, 7)
	verifier := newVerifierForTest(account, nil)

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), stubAuthTx{
		signers: [][]byte{address},
		sigs: []signingtypes.SignatureV2{
			{
				Sequence: 7,
				Data:     &signingtypes.MultiSignatureData{},
			},
		},
	}, false)

	require.ErrorContains(t, err, "mina transactions require single signature data")
}

// Missing registry entries should be reported as authorization failures.
// This is the bridge between Cosmos signer identity and Mina key resolution.
func TestMinaVerifierRejectsMissingRegistryEntry(t *testing.T) {
	t.Parallel()

	ctx, keyregistryKeeper := newKeyregistryKeeperForTest(t)
	_, account := newVerifierAccount(t, 1, 7)
	verifier := newVerifierForTest(account, keyregistryKeeper)

	err := verifier.VerifySignatures(ctx.WithIsSigverifyTx(true), stubAuthTx{
		signers: [][]byte{account.GetAddress()},
		sigs: []signingtypes.SignatureV2{
			{
				Sequence: 7,
				Data: &signingtypes.SingleSignatureData{
					SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
					Signature: []byte{1},
				},
			},
		},
	}, false)

	require.ErrorContains(t, err, "no Mina public key registered for signer")
}

// Mina-authenticated txs need an existing Cosmos public key to locate the registered Mina key.
// Without it the ante handler cannot bridge from signer address to keyregistry state.
func TestMinaVerifierRejectsMissingCosmosPublicKey(t *testing.T) {
	t.Parallel()

	ctx, keyregistryKeeper := newKeyregistryKeeperForTest(t)
	address := sdk.AccAddress([]byte("verifier-address-004"))
	account := authtypes.NewBaseAccount(address, nil, 1, 7)
	verifier := newVerifierForTest(account, keyregistryKeeper)

	err := verifier.VerifySignatures(ctx.WithIsSigverifyTx(true), stubAuthTx{
		signers: [][]byte{address},
		sigs: []signingtypes.SignatureV2{
			{
				Sequence: 7,
				Data: &signingtypes.SingleSignatureData{
					SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
					Signature: []byte{1},
				},
			},
		},
	}, false)

	require.ErrorIs(t, err, sdkerrors.ErrInvalidPubKey)
	require.ErrorContains(t, err, "no Cosmos public key found for signer")
}

// Keyregistry output must be a valid compressed Mina public key.
// Invalid bytes should fail before signature decoding or sign-byte generation.
func TestMinaVerifierRejectsInvalidRegisteredMinaPublicKey(t *testing.T) {
	t.Parallel()

	ctx, keyregistryKeeper := newKeyregistryKeeperForTest(t)
	_, account := newVerifierAccount(t, 1, 7)
	invalidMinaPubKey := make([]byte, keys.PublicKeyTotalByteSize)
	invalidMinaPubKey[keys.PublicKeyXByteSize] = 0x02
	registerMinaPublicKeyForAccount(t, ctx, keyregistryKeeper, account, invalidMinaPubKey)
	verifier := newVerifierForTest(account, keyregistryKeeper)

	err := verifier.VerifySignatures(ctx.WithIsSigverifyTx(true), stubAuthTx{
		signers: [][]byte{account.GetAddress()},
		sigs: []signingtypes.SignatureV2{
			{
				Sequence: 7,
				Data: &signingtypes.SingleSignatureData{
					SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
					Signature: []byte{1},
				},
			},
		},
	}, false)

	require.ErrorIs(t, err, sdkerrors.ErrInvalidPubKey)
	require.ErrorContains(t, err, "failed to parse Mina public key")
}

// Signature payloads must decode as real Mina signatures before any sign-byte comparison can happen.
// This keeps malformed wire data distinct from valid-but-unauthorized signatures.
func TestMinaVerifierRejectsInvalidSignatureEncoding(t *testing.T) {
	t.Parallel()

	ctx, keyregistryKeeper := newKeyregistryKeeperForTest(t)
	_, account := newVerifierAccount(t, 1, 7)
	minaPrivateKey := newMinaPrivateKeyForTest(t, 1)
	registerMinaPublicKeyForAccount(t, ctx, keyregistryKeeper, account, minaPublicKeyBytesForTest(t, minaPrivateKey))

	verifier := newVerifierForTest(account, keyregistryKeeper)

	err := verifier.VerifySignatures(ctx.WithIsSigverifyTx(true), stubAuthTx{
		signers: [][]byte{account.GetAddress()},
		sigs: []signingtypes.SignatureV2{
			{
				Sequence: 7,
				Data: &signingtypes.SingleSignatureData{
					SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
					Signature: []byte{1},
				},
			},
		},
	}, false)

	require.ErrorContains(t, err, "failed to parse Mina signature")
}

// Simulation mode skips expensive Mina crypto while leaving the outer verifier flow intact.
// Resolver should not be consulted because no actual signature check is meant to happen.
func TestMinaVerifierSkipsCryptoVerificationDuringSimulation(t *testing.T) {
	t.Parallel()

	address := sdk.AccAddress([]byte("verifier-address-006"))
	account := authtypes.NewBaseAccount(address, nil, 1, 7)
	verifier := newVerifierForTest(account, nil)

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), stubAuthTx{
		signers: [][]byte{address},
		sigs: []signingtypes.SignatureV2{
			{
				Sequence: 7,
				Data: &signingtypes.SingleSignatureData{
					SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
					Signature: []byte{1},
				},
			},
		},
	}, true)

	require.NoError(t, err)
}

// ReCheckTx skips expensive signature verification but still enforces cheap invariants.
// Resolver should not be called when the recheck flag is set.
func TestMinaVerifierSkipsCryptoVerificationDuringRecheck(t *testing.T) {
	t.Parallel()
	address := sdk.AccAddress([]byte("verifier-address-recheck"))
	account := authtypes.NewBaseAccount(address, nil, 1, 7)
	verifier := newVerifierForTest(account, nil)

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true).WithIsReCheckTx(true), stubAuthTx{
		signers: [][]byte{address},
		sigs: []signingtypes.SignatureV2{
			{
				Sequence: 7,
				Data: &signingtypes.SingleSignatureData{
					SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
					Signature: []byte{1},
				},
			},
		},
	}, false)

	require.NoError(t, err)
}

// Contexts that disable sigverify should bypass Mina crypto checks entirely.
// This branch is distinct from simulation and recheck, so it needs its own coverage.
func TestMinaVerifierSkipsCryptoVerificationWhenSigverifyDisabled(t *testing.T) {
	t.Parallel()
	address := sdk.AccAddress([]byte("verifier-address-nosigverify"))
	account := authtypes.NewBaseAccount(address, nil, 1, 7)
	verifier := newVerifierForTest(account, nil)

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(false), stubAuthTx{
		signers: [][]byte{address},
		sigs: []signingtypes.SignatureV2{
			{
				Sequence: 7,
				Data: &signingtypes.SingleSignatureData{
					SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
					Signature: []byte{1},
				},
			},
		},
	}, false)

	require.NoError(t, err)
}

// A decodable Mina signature is not enough if the sign mode has no registered handler.
// This covers the sign-bytes generation failure branch inside verifySingleSignature.
func TestMinaVerifierRejectsUnsupportedSignMode(t *testing.T) {
	t.Parallel()
	cosmosPrivKey, account := newVerifierAccount(t, 5, 9)
	minaPrivKey := newMinaPrivateKeyForTest(t, 21)
	registryCtx, keyregistryKeeper := newKeyregistryKeeperForTest(t)
	ctx := registryCtx.WithIsSigverifyTx(true)
	registerMinaPublicKeyForAccount(t, registryCtx, keyregistryKeeper, account, minaPublicKeyBytesForTest(t, minaPrivKey))
	verifier, encoding := newVerifierWithRealSignModeHandlerForTest(t, account, keyregistryKeeper)

	// The signature must decode successfully so the verifier reaches sign-bytes generation.
	signatureBytes := signMinaBytes(t, minaPrivKey, []byte("unsupported-sign-mode"), DefaultMinaNetworkID)
	tx := buildVerifierTestTx(
		t,
		encoding.TxConfig,
		account.GetAddress(),
		cosmosPrivKey.PubKey(),
		account.GetSequence(),
		signingtypes.SignMode(999),
		signatureBytes,
	)

	err := verifier.VerifySignatures(ctx, tx, false)

	require.ErrorIs(t, err, sdkerrors.ErrInvalidType)
	require.ErrorContains(t, err, "failed to generate sign bytes")
}

// The verifier must reject signatures that decode correctly but were made over the wrong message.
// This distinguishes actual cryptographic failure from malformed signature bytes.
func TestMinaVerifierRejectsCryptographicallyInvalidSignature(t *testing.T) {
	t.Parallel()
	cosmosPrivKey, account := newVerifierAccount(t, 6, 10)
	minaPrivKey := newMinaPrivateKeyForTest(t, 22)
	registryCtx, keyregistryKeeper := newKeyregistryKeeperForTest(t)
	ctx := registryCtx.WithIsSigverifyTx(true)
	registerMinaPublicKeyForAccount(t, registryCtx, keyregistryKeeper, account, minaPublicKeyBytesForTest(t, minaPrivKey))
	verifier, encoding := newVerifierWithRealSignModeHandlerForTest(t, account, keyregistryKeeper)

	signMode := signingtypes.SignMode(encoding.TxConfig.SignModeHandler().DefaultMode())
	// We first build the real tx shape so the invalid signature targets the exact bytes verifier expects.
	tx := buildVerifierTestTx(
		t,
		encoding.TxConfig,
		account.GetAddress(),
		cosmosPrivKey.PubKey(),
		account.GetSequence(),
		signMode,
		nil,
	)

	// The signature is valid for a different message, so decoding succeeds but verification fails.
	invalidSignatureBytes := signMinaBytes(t, minaPrivKey, []byte("different-sign-bytes"), DefaultMinaNetworkID)
	tx = buildVerifierTestTx(
		t,
		encoding.TxConfig,
		account.GetAddress(),
		cosmosPrivKey.PubKey(),
		account.GetSequence(),
		signMode,
		invalidSignatureBytes,
	)

	err := verifier.VerifySignatures(ctx, tx, false)

	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.ErrorContains(t, err, "Mina signature verification failed")
}

// A matching Mina key, real sign bytes and valid signature should pass end to end.
// This is the main success path for the verifier's custom crypto flow.
func TestMinaVerifierAcceptsValidSignature(t *testing.T) {
	t.Parallel()
	cosmosPrivKey, account := newVerifierAccount(t, 7, 11)
	minaPrivKey := newMinaPrivateKeyForTest(t, 23)
	registryCtx, keyregistryKeeper := newKeyregistryKeeperForTest(t)
	ctx := registryCtx.WithIsSigverifyTx(true)
	registerMinaPublicKeyForAccount(t, registryCtx, keyregistryKeeper, account, minaPublicKeyBytesForTest(t, minaPrivKey))
	verifier, encoding := newVerifierWithRealSignModeHandlerForTest(t, account, keyregistryKeeper)

	signMode := signingtypes.SignMode(encoding.TxConfig.SignModeHandler().DefaultMode())
	// The first tx instance gives us the exact bytes the verifier will later reconstruct.
	tx := buildVerifierTestTx(
		t,
		encoding.TxConfig,
		account.GetAddress(),
		cosmosPrivKey.PubKey(),
		account.GetSequence(),
		signMode,
		nil,
	)
	signBytes := buildVerifierSignBytes(t, ctx, encoding.TxConfig, tx, account, account.GetSequence(), signMode)
	signatureBytes := signMinaBytes(t, minaPrivKey, signBytes, DefaultMinaNetworkID)
	// Rebuild the tx with the real signature so VerifySignatures sees the same payload it validates.
	tx = buildVerifierTestTx(
		t,
		encoding.TxConfig,
		account.GetAddress(),
		cosmosPrivKey.PubKey(),
		account.GetSequence(),
		signMode,
		signatureBytes,
	)

	err := verifier.VerifySignatures(ctx, tx, false)

	require.NoError(t, err)
}

// At genesis height the verifier intentionally signs with account number zero.
// This test locks down that branch with a valid end-to-end signature.
func TestMinaVerifierUsesZeroAccountNumberAtGenesisHeight(t *testing.T) {
	t.Parallel()
	cosmosPrivKey, account := newVerifierAccount(t, 99, 12)
	minaPrivKey := newMinaPrivateKeyForTest(t, 24)
	registryCtx, keyregistryKeeper := newKeyregistryKeeperForTest(t)
	ctx := registryCtx.WithBlockHeight(0).WithIsSigverifyTx(true)
	registerMinaPublicKeyForAccount(t, registryCtx, keyregistryKeeper, account, minaPublicKeyBytesForTest(t, minaPrivKey))
	verifier, encoding := newVerifierWithRealSignModeHandlerForTest(t, account, keyregistryKeeper)

	signMode := signingtypes.SignMode(encoding.TxConfig.SignModeHandler().DefaultMode())
	// The sign bytes here intentionally omit the account number because block height is zero.
	tx := buildVerifierTestTx(
		t,
		encoding.TxConfig,
		account.GetAddress(),
		cosmosPrivKey.PubKey(),
		account.GetSequence(),
		signMode,
		nil,
	)
	signBytes := buildVerifierSignBytes(t, ctx, encoding.TxConfig, tx, account, account.GetSequence(), signMode)
	signatureBytes := signMinaBytes(t, minaPrivKey, signBytes, DefaultMinaNetworkID)
	// Rebuilding the tx with the real signature ensures the verifier sees the exact genesis-path payload.
	tx = buildVerifierTestTx(
		t,
		encoding.TxConfig,
		account.GetAddress(),
		cosmosPrivKey.PubKey(),
		account.GetSequence(),
		signMode,
		signatureBytes,
	)

	err := verifier.VerifySignatures(ctx, tx, false)

	require.NoError(t, err)
}
