package keeper_test

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	"github.com/node101-io/mina-signer-go/publickey"
	"github.com/node101-io/mina-signer-go/signature"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
)

// Mirrors the unexported network constant in the package under test.
const registrationNetworkIDForTest = mina.TestNet

// Captured from a REAL Auro Wallet (account
// B62qrRtUE4LbXmoYzkRVSHQ8pLBdfxEeNDKZU7dDkVUr2mrE1WAaZPw) through the exact
// call a bridge UI makes:
//
//	window.mina.signFields({ message: [challenge] })
//
// where challenge is RegistrationChallenge(actor, cosmosPublicKey) rendered as
// a decimal field element. Signatures were collected on both mina:devnet and
// mina:testnet and verify identically, which is the fork's TestNet/DevNet
// prefix case doing its job.
//
// Auro returns the signature base58-encoded; it is decoded and repacked here
// as field(32B) || scalar(32B), each LITTLE-endian — Mina's wire order, which
// the deserialiser requires (big-endian is rejected as non-canonical). The
// public key is packed as x in 32-byte little-endian with the odd-y flag in
// the top bit.
const (
	walletMinaPublicKeyB64  = "7fW+TcYCvStQ0ZYv1arI4MUF/aQ38xqc4TQfMokuvJs="
	walletUserSignatureB64  = "EN17Tzl+OTmGSg6L3tp0SgGD3xi2U2L0cWSh9qR2JBrrt1/+Qkm4pfLrMhVpdmSM42GDHeucmDb3rj/dMVsvAw=="
	walletValidatorSigB64   = "BLdkly0Thr7GdmFSFnincSTjssdrNZ+OPYwshHkvSDENCUFkxniIigOgxngXq0pxbVCZOvAZ6JIvmJdlaowKFA=="
	walletSignMessageSigB64 = "FQ7gjnIfKnEv0dat1dNmDMr0wC2cFYqifx5lcULoWylzTngslyTiKnFvIn2I/xhUQO45zEoQ2wS+NUG93YtaJg=="
	walletCosmosPubKeyHex   = "028e23b60777010732ad6bc2607f5ee5624fbba62ad284bc1300852cf90b2d94b0"
)

// TestWalletProducedSignatureVerifies is the reason registration signs a
// Poseidon CHALLENGE with signFields, rather than raw key bytes with
// signMessage/SignBytes.
//
// Two independent things break the byte path, and both stay invisible until a
// real wallet tries to register:
//
//   - deriving the signature domain from the actor type yields a prefix
//     ("USERSignature*******" instead of "CodaSignature*******") that no
//     wallet can be asked to sign under, since a wallet takes its network id
//     from the network it is connected to;
//   - o1js's signMessage uses the legacy ROInput packing, which
//     mina-signer-go's SignBytes/SignString do not reproduce — a signature
//     made one way never verifies the other, whatever the domain.
//
// The field-element path is the one both libraries implement identically.
// If this test fails, wallet-based registration has broken again and the only
// way back in is asking users to paste a raw private key.
func TestWalletProducedSignatureVerifies(t *testing.T) {
	cosmosPublicKey, err := hex.DecodeString(walletCosmosPubKeyHex)
	require.NoError(t, err)
	publicKey, err := decodeWalletKey(t)
	require.NoError(t, err)

	for _, tc := range []struct {
		actor  types.ActorType
		sigB64 string
	}{
		{types.ActorType_USER, walletUserSignatureB64},
		{types.ActorType_VALIDATOR, walletValidatorSigB64},
	} {
		challenge, err := types.RegistrationChallenge(tc.actor, cosmosPublicKey)
		require.NoError(t, err)

		sig, err := decodeSignature(tc.sigB64)
		require.NoError(t, err)

		valid, err := publicKey.VerifyField(sig, challenge)
		require.NoError(t, err)
		require.True(t, valid, "a real wallet signature must verify for actor %s", tc.actor)
	}
}

// TestWalletSignatureIsActorBound proves the Poseidon prefix carries the
// separation the actor-typed network id was reaching for: the signature a user
// produces is rejected as a validator registration, and the reverse.
func TestWalletSignatureIsActorBound(t *testing.T) {
	cosmosPublicKey, err := hex.DecodeString(walletCosmosPubKeyHex)
	require.NoError(t, err)
	publicKey, err := decodeWalletKey(t)
	require.NoError(t, err)

	userChallenge, err := types.RegistrationChallenge(types.ActorType_USER, cosmosPublicKey)
	require.NoError(t, err)

	validatorSig, err := decodeSignature(walletValidatorSigB64)
	require.NoError(t, err)

	require.False(t,
		verifies(publicKey.VerifyField(validatorSig, userChallenge)),
		"a validator registration signature must not authorise a user registration")
}

// TestWalletSignMessageIsUnusable is the negative half of the story, kept
// because it is the finding that forced the field-element path: the other
// call a wallet offers, signMessage, produces a signature no chain-side
// verifier accepts — not VerifyField, not VerifyBytes, not VerifyString. The
// packing behind signMessage is Mina's legacy scheme, which mina-signer-go
// does not implement, so "just sign the bytes" is not an option however the
// domain is chosen.
func TestWalletSignMessageIsUnusable(t *testing.T) {
	cosmosPublicKey, err := hex.DecodeString(walletCosmosPubKeyHex)
	require.NoError(t, err)
	publicKey, err := decodeWalletKey(t)
	require.NoError(t, err)

	challenge, err := types.RegistrationChallenge(types.ActorType_USER, cosmosPublicKey)
	require.NoError(t, err)

	sig, err := decodeSignature(walletSignMessageSigB64)
	require.NoError(t, err)

	require.False(t, verifies(publicKey.VerifyField(sig, challenge)), "VerifyField")
	require.False(t, verifies(publicKey.VerifyBytes(sig, cosmosPublicKey)), "VerifyBytes")
	require.False(t, verifies(publicKey.VerifyString(sig, walletCosmosPubKeyHex)), "VerifyString")
}

// verifies collapses the library's two ways of saying "no": a false result
// and a verification error. Callers asserting a signature is REJECTED must
// not care which one they get.
func verifies(valid bool, err error) bool {
	return err == nil && valid
}

func decodeWalletKey(t *testing.T) (*publickey.PublicKey, error) {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(walletMinaPublicKeyB64)
	require.NoError(t, err)
	return publickey.NewPublicKeyFromBytes(raw, registrationNetworkIDForTest)
}

func decodeSignature(b64 string) (*signature.Signature, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	return signature.NewSignatureFromBytes(raw)
}

// TestRegistrationChallengeIsInjective covers what the byte packing has to
// guarantee: distinct inputs must not share a challenge, or one registration's
// signature would authorise another. The stop-byte chunking makes length part
// of the encoding, so a shorter key cannot collide with a longer one that
// merely starts with the same bytes.
func TestRegistrationChallengeIsInjective(t *testing.T) {
	userKey, err := hex.DecodeString(walletCosmosPubKeyHex)
	require.NoError(t, err)

	challengeOf := func(actor types.ActorType, key []byte) string {
		c, err := types.RegistrationChallenge(actor, key)
		require.NoError(t, err)
		return string(c.Bytes())
	}

	base := challengeOf(types.ActorType_USER, userKey)

	// Same key, other actor: separated by the Poseidon prefix.
	require.NotEqual(t, base, challengeOf(types.ActorType_VALIDATOR, userKey))

	// A prefix of the key, and the key with a zero appended — the two shapes a
	// length-blind packing would fold together.
	require.NotEqual(t, base, challengeOf(types.ActorType_USER, userKey[:len(userKey)-1]))
	require.NotEqual(t, base, challengeOf(types.ActorType_USER, append(append([]byte{}, userKey...), 0x00)))

	// Every byte is bound, including across the 31-byte chunk boundary.
	for _, index := range []int{0, 30, 31, len(userKey) - 1} {
		mutated := append([]byte{}, userKey...)
		mutated[index] ^= 0x01
		require.NotEqual(t, base, challengeOf(types.ActorType_USER, mutated), "flipping byte %d must change the challenge", index)
	}

	_, err = types.RegistrationChallenge(types.ActorType_UNSPECIFIED, userKey)
	require.ErrorIs(t, err, types.ErrInvalidActorType)
}
