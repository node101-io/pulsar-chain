package ante

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	keyregistrytypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func TestBuildTxSigningChallengeIsDeterministic(t *testing.T) {
	first, err := buildTxSigningChallenge([]byte("payload"))
	require.NoError(t, err)
	second, err := buildTxSigningChallenge([]byte("payload"))
	require.NoError(t, err)

	require.Equal(t, first.Bytes(), second.Bytes())
}

func TestBuildTxSigningChallengeSeparatesPayloads(t *testing.T) {
	first, err := buildTxSigningChallenge([]byte("payload-a"))
	require.NoError(t, err)
	second, err := buildTxSigningChallenge([]byte("payload-b"))
	require.NoError(t, err)

	require.NotEqual(t, first.Bytes(), second.Bytes())
}

func TestBuildTxSigningChallengeRejectsEmptySignBytes(t *testing.T) {
	_, err := buildTxSigningChallenge(nil)
	require.Error(t, err)
	_, err = buildTxSigningChallenge([]byte{})
	require.Error(t, err)
}

// This golden vector detects accidental wire-format drift. It is not an
// independent cross-language or wallet interoperability proof.
func TestBuildTxSigningChallengeVector(t *testing.T) {
	challenge, err := buildTxSigningChallenge([]byte("pulsar-tx-vector-01"))
	require.NoError(t, err)

	require.Equal(t,
		"185cf93846576559d5abe9e6c8b5d637940727c9516d1f346a775fe57d897cff",
		hex.EncodeToString(challenge.Bytes()),
	)
}

// A key-registration proof must not authorize a transaction, even when its
// challenge bytes are reused as the transaction derivation input.
func TestKeyRegistrationSignatureCannotAuthorizeTransaction(t *testing.T) {
	cosmosKey := make([]byte, 33)
	cosmosKey[0] = 0x02
	minaPrivateKey := newMinaPrivateKeyForTest(t, 0x07)
	minaKey := minaPublicKeyBytesForTest(t, minaPrivateKey)

	registration, err := keyregistrytypes.BuildKeySigningChallenge(keyregistrytypes.KeySigningChallengeInput{
		ChainID:          "mytestnet",
		Operation:        keyregistrytypes.KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER,
		ActorType:        keyregistrytypes.ActorType_ACTOR_TYPE_USER,
		CosmosPublicKey:  cosmosKey,
		NewMinaPublicKey: minaKey,
	})
	require.NoError(t, err)
	registrationSignature, err := minaPrivateKey.SignFieldElement(registration)
	require.NoError(t, err)

	txChallenge, err := buildTxSigningChallenge(registration.Bytes())
	require.NoError(t, err)
	minaPublicKey, err := minaPrivateKey.ToPublicKey()
	require.NoError(t, err)

	valid, err := minaPublicKey.VerifyField(registrationSignature, txChallenge)
	require.Error(t, err)
	require.False(t, valid)
}
