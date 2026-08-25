package ante

import (
	"bytes"
	"context"
	"errors"
	"testing"

	txsigning "cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"

	antetypes "github.com/node101-io/pulsar-chain/app/ante/types"
	smartaccountstypes "github.com/node101-io/pulsar-chain/x/smartaccounts/types"
)

type recordingSmartAccountKeeper struct {
	authorized     bool
	err            error
	calls          int
	identity       []byte
	accountAddress []byte
	publicKey      []byte
}

func (k *recordingSmartAccountKeeper) IsSessionKeyAuthorized(
	_ context.Context,
	identity, accountAddress, publicKey []byte,
) (bool, error) {
	k.calls++
	k.identity = append([]byte(nil), identity...)
	k.accountAddress = append([]byte(nil), accountAddress...)
	k.publicKey = append([]byte(nil), publicKey...)

	return k.authorized, k.err
}

type stubSmartAccountTx struct {
	stubAuthTx
	extensionOptions []*codectypes.Any
}

func (tx stubSmartAccountTx) GetExtensionOptions() []*codectypes.Any {
	return tx.extensionOptions
}

func mustSmartAccountExtensionAny(t *testing.T, identity []byte) *codectypes.Any {
	t.Helper()

	bz, err := gogoproto.Marshal(&antetypes.TxAuthModeExtension{
		TxAuthMode:           antetypes.TX_AUTH_MODE_SMART_ACCOUNT,
		SmartAccountIdentity: identity,
	})
	require.NoError(t, err)

	return &codectypes.Any{
		TypeUrl: txAuthModeExtensionTypeURL,
		Value:   bz,
	}
}

func buildSmartAccountVerifierTestTx(
	t *testing.T,
	txConfig client.TxConfig,
	from sdk.AccAddress,
	pubKey cryptotypes.PubKey,
	identity []byte,
	sequence uint64,
	signature []byte,
) sdk.Tx {
	t.Helper()

	builder := txConfig.NewTxBuilder()
	to := sdk.AccAddress([]byte("smart-account-recipient"))

	err := builder.SetMsgs(banktypes.NewMsgSend(
		from,
		to,
		sdk.NewCoins(sdk.NewInt64Coin("pmina", 1)),
	))
	require.NoError(t, err)

	builder.SetGasLimit(200000)
	builder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("pmina", 1)))
	builder.SetMemo("smart-account-verifier-test")

	extendedBuilder, ok := builder.(client.ExtendedTxBuilder)
	require.True(t, ok)
	extendedBuilder.SetExtensionOptions(mustSmartAccountExtensionAny(t, identity))

	err = builder.SetSignatures(signingtypes.SignatureV2{
		PubKey: pubKey,
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
			Signature: signature,
		},
		Sequence: sequence,
	})
	require.NoError(t, err)

	return builder.GetTx()
}

func newStubSmartAccountTx(
	t *testing.T,
	accountAddress, identity []byte,
	pubKey cryptotypes.PubKey,
	sequence uint64,
) stubSmartAccountTx {
	t.Helper()

	return stubSmartAccountTx{
		stubAuthTx: stubAuthTx{
			signers: [][]byte{accountAddress},
			sigs: []signingtypes.SignatureV2{
				{
					PubKey:   pubKey,
					Sequence: sequence,
					Data: &signingtypes.SingleSignatureData{
						SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
						Signature: []byte{1},
					},
				},
			},
		},
		extensionOptions: []*codectypes.Any{
			mustSmartAccountExtensionAny(t, identity),
		},
	}
}

func TestSmartAccountVerifierAcceptsValidSignature(t *testing.T) {
	t.Parallel()

	_, account := newVerifierAccount(t, 7, 11)
	sessionPrivateKey := ed25519.GenPrivKey()
	identity := bytes.Repeat([]byte{1}, smartaccountstypes.IdentitySize)
	keeper := &recordingSmartAccountKeeper{authorized: true}
	encoding := newVerifierEncodingConfig(t)
	verifier := NewSmartAccountVerifier(
		keeper,
		verifierAccountKeeper{account: account},
		encoding.TxConfig.SignModeHandler(),
	)
	ctx := newTestSDKContext(t).WithIsSigverifyTx(true)

	unsignedTx := buildSmartAccountVerifierTestTx(
		t,
		encoding.TxConfig,
		account.GetAddress(),
		sessionPrivateKey.PubKey(),
		identity,
		account.GetSequence(),
		nil,
	)
	signBytes := buildVerifierSignBytes(
		t,
		ctx,
		encoding.TxConfig,
		unsignedTx,
		account,
		account.GetSequence(),
		signingtypes.SignMode_SIGN_MODE_DIRECT,
	)
	signature, err := sessionPrivateKey.Sign(signBytes)
	require.NoError(t, err)

	signedTx := buildSmartAccountVerifierTestTx(
		t,
		encoding.TxConfig,
		account.GetAddress(),
		sessionPrivateKey.PubKey(),
		identity,
		account.GetSequence(),
		signature,
	)

	err = verifier.VerifySignatures(ctx, signedTx, false)

	require.NoError(t, err)
	require.Equal(t, 1, keeper.calls)
	require.Equal(t, identity, keeper.identity)
	require.Equal(t, []byte(account.GetAddress()), keeper.accountAddress)
	require.Equal(t, sessionPrivateKey.PubKey().Bytes(), keeper.publicKey)
}

func TestSmartAccountVerifierRejectsUnauthorizedSessionKey(t *testing.T) {
	t.Parallel()

	_, account := newVerifierAccount(t, 1, 4)
	sessionPrivateKey := ed25519.GenPrivKey()
	identity := bytes.Repeat([]byte{2}, smartaccountstypes.IdentitySize)
	keeper := &recordingSmartAccountKeeper{authorized: false}
	verifier := NewSmartAccountVerifier(
		keeper,
		verifierAccountKeeper{account: account},
		&txsigning.HandlerMap{},
	)
	tx := newStubSmartAccountTx(
		t,
		account.GetAddress(),
		identity,
		sessionPrivateKey.PubKey(),
		account.GetSequence(),
	)

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), tx, false)

	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.Equal(t, 1, keeper.calls)
}

func TestSmartAccountVerifierPropagatesKeeperError(t *testing.T) {
	t.Parallel()

	_, account := newVerifierAccount(t, 1, 4)
	sessionPrivateKey := ed25519.GenPrivKey()
	identity := bytes.Repeat([]byte{3}, smartaccountstypes.IdentitySize)
	expectedErr := errors.New("smart account state unavailable")
	keeper := &recordingSmartAccountKeeper{err: expectedErr}
	verifier := NewSmartAccountVerifier(
		keeper,
		verifierAccountKeeper{account: account},
		&txsigning.HandlerMap{},
	)
	tx := newStubSmartAccountTx(
		t,
		account.GetAddress(),
		identity,
		sessionPrivateKey.PubKey(),
		account.GetSequence(),
	)

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), tx, false)

	require.ErrorIs(t, err, expectedErr)
}

func TestSmartAccountVerifierRejectsWrongSequence(t *testing.T) {
	t.Parallel()

	_, account := newVerifierAccount(t, 1, 4)
	sessionPrivateKey := ed25519.GenPrivKey()
	identity := bytes.Repeat([]byte{4}, smartaccountstypes.IdentitySize)
	keeper := &recordingSmartAccountKeeper{authorized: true}
	verifier := NewSmartAccountVerifier(
		keeper,
		verifierAccountKeeper{account: account},
		&txsigning.HandlerMap{},
	)
	tx := newStubSmartAccountTx(
		t,
		account.GetAddress(),
		identity,
		sessionPrivateKey.PubKey(),
		account.GetSequence()+1,
	)

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), tx, false)

	require.ErrorIs(t, err, sdkerrors.ErrWrongSequence)
}

func TestSmartAccountVerifierRejectsInvalidSignature(t *testing.T) {
	t.Parallel()

	_, account := newVerifierAccount(t, 5, 8)
	sessionPrivateKey := ed25519.GenPrivKey()
	identity := bytes.Repeat([]byte{5}, smartaccountstypes.IdentitySize)
	keeper := &recordingSmartAccountKeeper{authorized: true}
	encoding := newVerifierEncodingConfig(t)
	verifier := NewSmartAccountVerifier(
		keeper,
		verifierAccountKeeper{account: account},
		encoding.TxConfig.SignModeHandler(),
	)
	tx := buildSmartAccountVerifierTestTx(
		t,
		encoding.TxConfig,
		account.GetAddress(),
		sessionPrivateKey.PubKey(),
		identity,
		account.GetSequence(),
		make([]byte, ed25519.SignatureSize),
	)

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), tx, false)

	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.ErrorContains(t, err, "signature verification failed")
}

func TestSmartAccountVerifierSkipsCryptoVerificationDuringSimulation(t *testing.T) {
	t.Parallel()

	_, account := newVerifierAccount(t, 1, 4)
	sessionPrivateKey := ed25519.GenPrivKey()
	identity := bytes.Repeat([]byte{6}, smartaccountstypes.IdentitySize)
	keeper := &recordingSmartAccountKeeper{authorized: true}
	verifier := NewSmartAccountVerifier(
		keeper,
		verifierAccountKeeper{account: account},
		&txsigning.HandlerMap{},
	)
	tx := newStubSmartAccountTx(
		t,
		account.GetAddress(),
		identity,
		sessionPrivateKey.PubKey(),
		account.GetSequence(),
	)

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), tx, true)

	require.NoError(t, err)
}

func TestSmartAccountVerifierRejectsInvalidEnvelope(t *testing.T) {
	t.Parallel()

	identity := bytes.Repeat([]byte{7}, smartaccountstypes.IdentitySize)
	accountAddress := sdk.AccAddress([]byte("smart-account-signer"))
	account := authtypes.NewBaseAccount(accountAddress, nil, 1, 4)
	sessionPrivateKey := ed25519.GenPrivKey()
	keeper := &recordingSmartAccountKeeper{authorized: true}
	verifier := NewSmartAccountVerifier(
		keeper,
		verifierAccountKeeper{account: account},
		&txsigning.HandlerMap{},
	)

	tests := []struct {
		name string
		tx   sdk.Tx
		want error
	}{
		{
			name: "invalid tx type",
			tx:   stubBasicTx{},
			want: sdkerrors.ErrTxDecode,
		},
		{
			name: "unordered tx",
			tx: stubSmartAccountTx{stubAuthTx: stubAuthTx{
				unordered: true,
			}},
			want: sdkerrors.ErrNotSupported,
		},
		{
			name: "missing signature",
			tx: stubSmartAccountTx{
				stubAuthTx: stubAuthTx{signers: [][]byte{accountAddress}},
			},
			want: sdkerrors.ErrUnauthorized,
		},
		{
			name: "non ed25519 public key",
			tx: newStubSmartAccountTx(
				t,
				accountAddress,
				identity,
				secp256k1.GenPrivKey().PubKey(),
				account.GetSequence(),
			),
			want: sdkerrors.ErrInvalidPubKey,
		},
		{
			name: "unsupported sign mode",
			tx: stubSmartAccountTx{
				stubAuthTx: stubAuthTx{
					signers: [][]byte{accountAddress},
					sigs: []signingtypes.SignatureV2{
						{
							PubKey:   sessionPrivateKey.PubKey(),
							Sequence: account.GetSequence(),
							Data: &signingtypes.SingleSignatureData{
								SignMode: signingtypes.SignMode_SIGN_MODE_DIRECT_AUX,
							},
						},
					},
				},
				extensionOptions: []*codectypes.Any{
					mustSmartAccountExtensionAny(t, identity),
				},
			},
			want: sdkerrors.ErrNotSupported,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), test.tx, false)
			require.ErrorIs(t, err, test.want)
		})
	}
}

func TestSmartAccountIdentityFromTx(t *testing.T) {
	t.Parallel()

	identity := bytes.Repeat([]byte{8}, smartaccountstypes.IdentitySize)
	minaExtension := mustTxAuthModeExtensionAny(t, antetypes.TX_AUTH_MODE_MINA)

	tests := []struct {
		name    string
		tx      sdk.Tx
		want    []byte
		wantErr error
	}{
		{
			name: "valid identity",
			tx: stubSmartAccountTx{extensionOptions: []*codectypes.Any{
				mustSmartAccountExtensionAny(t, identity),
			}},
			want: identity,
		},
		{
			name:    "tx without extension support",
			tx:      stubAuthTx{},
			wantErr: sdkerrors.ErrInvalidRequest,
		},
		{
			name:    "missing auth extension",
			tx:      stubSmartAccountTx{},
			wantErr: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "wrong auth mode",
			tx: stubSmartAccountTx{extensionOptions: []*codectypes.Any{
				minaExtension,
			}},
			wantErr: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "invalid identity length",
			tx: stubSmartAccountTx{extensionOptions: []*codectypes.Any{
				mustSmartAccountExtensionAny(t, identity[:len(identity)-1]),
			}},
			wantErr: sdkerrors.ErrInvalidRequest,
		},
		{
			name: "duplicate auth extension",
			tx: stubSmartAccountTx{extensionOptions: []*codectypes.Any{
				mustSmartAccountExtensionAny(t, identity),
				mustSmartAccountExtensionAny(t, identity),
			}},
			wantErr: sdkerrors.ErrInvalidRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := smartAccountIdentityFromTx(test.tx)
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.want, actual)
		})
	}
}
