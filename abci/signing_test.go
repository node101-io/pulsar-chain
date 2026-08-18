package abci

import (
	"bytes"
	"errors"
	"testing"

	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/mina-signer-go/privatekey"
	votepersistencetypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
	"github.com/stretchr/testify/require"
)

func TestSignVoteExtensionRoundTrip(t *testing.T) {
	secondaryKey := validSecondaryKey()
	body := validVoteExtBody()
	proofCommitment := testProofCommitment()

	signature, err := secondaryKey.SignVoteExtension(body, proofCommitment)
	require.NoError(t, err)
	require.NotEmpty(t, signature)

	minaPublicKey := secondaryKey.PublicKey.Bytes()

	poseidonHash := testPoseidonHash()
	require.NoError(t, verifyVoteExtSig(
		poseidonHash,
		signature,
		body,
		proofCommitment,
		minaPublicKey,
		NetworkID,
	))
}

func TestVerifyVoteExtSigFailureModes(t *testing.T) {
	secondaryKey := validSecondaryKey()
	body := validVoteExtBody()
	proofCommitment := testProofCommitment()
	signature, err := secondaryKey.SignVoteExtension(body, proofCommitment)
	require.NoError(t, err)
	minaPublicKey := secondaryKey.PublicKey.Bytes()

	tests := []struct {
		name        string
		poseidon    *poseidon.Poseidon
		signature   []byte
		message     votepersistencetypes.VoteExtBody
		minaKey     []byte
		expectedErr error
	}{
		{
			name:        "malformed mina public key",
			poseidon:    testPoseidonHash(),
			signature:   signature,
			message:     body,
			minaKey:     []byte("not-a-mina-public-key"),
			expectedErr: ErrInvalidVoteExtMinaPublicKey,
		},
		{
			name:        "malformed signature",
			poseidon:    testPoseidonHash(),
			signature:   []byte("not-a-signature"),
			message:     body,
			minaKey:     minaPublicKey,
			expectedErr: ErrInvalidVoteExtSignatureEncoding,
		},
		{
			name:        "nil poseidon hasher",
			poseidon:    nil,
			signature:   signature,
			message:     body,
			minaKey:     minaPublicKey,
			expectedErr: ErrVoteExtBodyHashFailed,
		},
		{
			name:      "signature does not match body",
			poseidon:  testPoseidonHash(),
			signature: signature,
			message: votepersistencetypes.VoteExtBody{
				NextValidatorSetHash: body.NextValidatorSetHash,
				CurrentStateRoot:     body.CurrentStateRoot,
				CurrentBlockHeight:   body.CurrentBlockHeight + 1,
				ActionsReducedRoot:   body.ActionsReducedRoot,
			},
			minaKey:     minaPublicKey,
			expectedErr: ErrInvalidVoteExtSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyVoteExtSig(
				tt.poseidon,
				tt.signature,
				tt.message,
				proofCommitment,
				tt.minaKey,
				NetworkID,
			)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func testProofCommitment() []byte {
	return bytes.Repeat([]byte{0x01}, proofCommitmentLength)
}

func marshalVoteExtensionForTest(
	t *testing.T,
	signature []byte,
	proofCommitment []byte,
) []byte {
	t.Helper()

	bz, err := (&VoteExtension{
		Signature:       signature,
		ProofCommitment: proofCommitment,
	}).Marshal()
	require.NoError(t, err)

	return bz
}

func TestSecondaryKeyValidate(t *testing.T) {
	validKey := validSecondaryKey()
	require.NoError(t, validKey.Validate())

	require.ErrorIs(t, (SecondaryKey{}).Validate(), ErrMissingSecondaryKey)

	otherPrivateKey, err := privatekey.NewPrivateKeyFromBytes([32]byte{
		2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2,
	}, NetworkID)
	require.NoError(t, err)
	otherPublicKey, err := otherPrivateKey.ToPublicKey()
	require.NoError(t, err)
	mismatchedKey := SecondaryKey{
		SecretKey: validKey.SecretKey,
		PublicKey: otherPublicKey,
	}

	err = mismatchedKey.Validate()
	require.True(t, errors.Is(err, ErrInvalidSecondaryKey))
}

func validSecondaryKey() SecondaryKey {
	privateKey, err := privatekey.NewPrivateKeyFromBytes([32]byte{
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
	}, NetworkID)
	if err != nil {
		panic(err)
	}
	publicKey, err := privateKey.ToPublicKey()
	if err != nil {
		panic(err)
	}

	return SecondaryKey{
		SecretKey: privateKey,
		PublicKey: publicKey,
	}
}

func testStateRoot32() []byte {
	return bytes.Repeat([]byte{0x42}, 32)
}

func validVoteExtBody() votepersistencetypes.VoteExtBody {
	return votepersistencetypes.VoteExtBody{
		NextValidatorSetHash: field.NewField().FromUint64(1).Bytes(),
		CurrentStateRoot:     testStateRoot32(),
		CurrentBlockHeight:   7,
		ActionsReducedRoot:   testActionsReducedRoot(),
	}
}

func testPoseidonHash() *poseidon.Poseidon {
	return poseidon.NewPoseidon()
}
