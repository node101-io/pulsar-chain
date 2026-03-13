package ante

import (
	"context"
	"errors"
	"testing"
	"time"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	txsigning "cosmossdk.io/x/tx/signing"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/node101-io/mina-signer-go/keys"
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

type verifierResolver struct {
	minaAddress []byte
	err         error
	calls       int
}

func (r *verifierResolver) GetCosmosToMina(context.Context, []byte) ([]byte, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}

	return r.minaAddress, nil
}

type stubAuthTx struct {
	signers   [][]byte
	sigs      []signingtypes.SignatureV2
	unordered bool
}

func (tx stubAuthTx) GetMsgs() []sdk.Msg { return nil }

func (tx stubAuthTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func (tx stubAuthTx) GetSigners() ([][]byte, error) { return tx.signers, nil }

func (tx stubAuthTx) GetPubKeys() ([]cryptotypes.PubKey, error) { return nil, nil }

func (tx stubAuthTx) GetSignaturesV2() ([]signingtypes.SignatureV2, error) { return tx.sigs, nil }

func (tx stubAuthTx) GetMemo() string { return "" }

func (tx stubAuthTx) GetTimeoutHeight() uint64 { return 0 }

func (tx stubAuthTx) GetTimeoutTimeStamp() time.Time { return time.Time{} }

func (tx stubAuthTx) GetUnordered() bool { return tx.unordered }

func (tx stubAuthTx) ValidateBasic() error { return nil }

func (tx stubAuthTx) GetGas() uint64 { return 0 }

func (tx stubAuthTx) GetFee() sdk.Coins { return nil }

func (tx stubAuthTx) FeePayer() []byte { return nil }

func (tx stubAuthTx) FeeGranter() []byte { return nil }

func newVerifierForTest(
	account sdk.AccountI,
	resolver *verifierResolver,
) MinaVerifier {
	return NewMinaVerifier(
		resolver,
		verifierAccountKeeper{account: account},
		&txsigning.HandlerMap{},
		DefaultMinaNetworkID,
		log.NewNopLogger(),
	)
}

func TestMinaVerifierRejectsUnorderedTransactions(t *testing.T) {
	t.Parallel()

	address := sdk.AccAddress([]byte("verifier-address-001"))
	account := authtypes.NewBaseAccount(address, nil, 1, 7)
	verifier := newVerifierForTest(account, &verifierResolver{})

	err := verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), stubAuthTx{
		unordered: true,
	}, false)

	require.ErrorContains(t, err, "unordered Mina transactions are not supported")
}

func TestMinaVerifierRejectsWrongSequence(t *testing.T) {
	t.Parallel()

	address := sdk.AccAddress([]byte("verifier-address-002"))
	account := authtypes.NewBaseAccount(address, nil, 1, 7)
	verifier := newVerifierForTest(account, &verifierResolver{})

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

func TestMinaVerifierRejectsNonSingleSignatureData(t *testing.T) {
	t.Parallel()

	address := sdk.AccAddress([]byte("verifier-address-003"))
	account := authtypes.NewBaseAccount(address, nil, 1, 7)
	verifier := newVerifierForTest(account, &verifierResolver{})

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

func TestMinaVerifierRejectsMissingRegistryEntry(t *testing.T) {
	t.Parallel()

	address := sdk.AccAddress([]byte("verifier-address-004"))
	account := authtypes.NewBaseAccount(address, nil, 1, 7)
	resolver := &verifierResolver{err: errors.New("not found")}
	verifier := newVerifierForTest(account, resolver)

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
	}, false)

	require.ErrorContains(t, err, "no Mina address registered for signer")
	require.Equal(t, 1, resolver.calls)
}

func TestMinaVerifierRejectsInvalidSignatureEncoding(t *testing.T) {
	t.Parallel()

	address := sdk.AccAddress([]byte("verifier-address-005"))
	account := authtypes.NewBaseAccount(address, nil, 1, 7)

	var seed [32]byte
	seed[0] = 1
	minaAddress, err := keys.NewPrivateKeyFromBytes(seed).ToPublicKey().ToAddress()
	require.NoError(t, err)

	resolver := &verifierResolver{minaAddress: []byte(minaAddress)}
	verifier := newVerifierForTest(account, resolver)

	err = verifier.VerifySignatures(newTestSDKContext(t).WithIsSigverifyTx(true), stubAuthTx{
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

	require.ErrorContains(t, err, "failed to parse Mina signature")
	require.Equal(t, 1, resolver.calls)
}

func TestMinaVerifierSkipsCryptoVerificationDuringSimulation(t *testing.T) {
	t.Parallel()

	address := sdk.AccAddress([]byte("verifier-address-006"))
	account := authtypes.NewBaseAccount(address, nil, 1, 7)
	resolver := &verifierResolver{err: errors.New("should not be called")}
	verifier := newVerifierForTest(account, resolver)

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
	require.Zero(t, resolver.calls)
}
