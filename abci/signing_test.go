package abci

import (
	"errors"
	"testing"

	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/mina-signer-go/privatekey"
	votepersistencetypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
	"github.com/stretchr/testify/require"
)

func TestSignVoteExtBodyRoundTrip(t *testing.T) {
	secondaryKey := validSecondaryKey()
	body := validVoteExtBody()

	signature, err := secondaryKey.SignVoteExtBody(body)
	require.NoError(t, err)
	require.NotEmpty(t, signature)

	minaPublicKey := secondaryKey.PublicKey.Bytes()

	poseidonHash := testPoseidonHash()
	require.NoError(t, verifyVoteExtSig(poseidonHash, signature, body, minaPublicKey, ActionsReducedRoot, NetworkID))
}

func TestVerifyVoteExtSigFailureModes(t *testing.T) {
	secondaryKey := validSecondaryKey()
	body := validVoteExtBody()
	signature, err := secondaryKey.SignVoteExtBody(body)
	require.NoError(t, err)
	minaPublicKey := secondaryKey.PublicKey.Bytes()

	tests := []struct {
		name        string
		poseidon    *poseidon.Poseidon
		signature   []byte
		message     votepersistencetypes.VoteExtBody
		minaKey     []byte
		reducedRoot string
		expectedErr error
	}{
		{
			name:        "wrong reduced root",
			poseidon:    testPoseidonHash(),
			signature:   signature,
			message:     body,
			minaKey:     minaPublicKey,
			reducedRoot: "wrong-root",
			expectedErr: ErrInvalidVoteExtReducedRoot,
		},
		{
			name:        "malformed mina public key",
			poseidon:    testPoseidonHash(),
			signature:   signature,
			message:     body,
			minaKey:     []byte("not-a-mina-public-key"),
			reducedRoot: ActionsReducedRoot,
			expectedErr: ErrInvalidVoteExtMinaPublicKey,
		},
		{
			name:        "malformed signature",
			poseidon:    testPoseidonHash(),
			signature:   []byte("not-a-signature"),
			message:     body,
			minaKey:     minaPublicKey,
			reducedRoot: ActionsReducedRoot,
			expectedErr: ErrInvalidVoteExtSignatureEncoding,
		},
		{
			name:        "nil poseidon hasher",
			poseidon:    nil,
			signature:   signature,
			message:     body,
			minaKey:     minaPublicKey,
			reducedRoot: ActionsReducedRoot,
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
			reducedRoot: ActionsReducedRoot,
			expectedErr: ErrInvalidVoteExtSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyVoteExtSig(tt.poseidon, tt.signature, tt.message, tt.minaKey, tt.reducedRoot, NetworkID)

			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
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

func validVoteExtBody() votepersistencetypes.VoteExtBody {
	return votepersistencetypes.VoteExtBody{
		NextValidatorSetHash: []byte("next-validator-set-hash"),
		CurrentStateRoot:     []byte("current-state-root"),
		CurrentBlockHeight:   7,
		ActionsReducedRoot:   ActionsReducedRoot,
	}
}

func testPoseidonHash() *poseidon.Poseidon {
	return poseidon.NewPoseidon()
}
