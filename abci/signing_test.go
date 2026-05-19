package abci

import (
	"errors"
	"math/big"
	"testing"

	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	votepersistencetypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
	"github.com/stretchr/testify/require"
)

func TestSignVoteExtBodyRoundTrip(t *testing.T) {
	secondaryKey := validSecondaryKey()
	body := validVoteExtBody()

	signature, err := secondaryKey.SignVoteExtBody(body)
	require.NoError(t, err)
	require.NotEmpty(t, signature)

	minaPublicKey, err := secondaryKey.PublicKey.MarshalBytes()
	require.NoError(t, err)

	poseidonHash := testPoseidonHash()
	require.NoError(t, verifyVoteExtSig(poseidonHash, signature, body, minaPublicKey, ActionsReducedRoot))
}

func TestVerifyVoteExtSigFailureModes(t *testing.T) {
	secondaryKey := validSecondaryKey()
	body := validVoteExtBody()
	signature, err := secondaryKey.SignVoteExtBody(body)
	require.NoError(t, err)
	minaPublicKey, err := secondaryKey.PublicKey.MarshalBytes()
	require.NoError(t, err)

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
			err := verifyVoteExtSig(tt.poseidon, tt.signature, tt.message, tt.minaKey, tt.reducedRoot)

			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestSecondaryKeyValidate(t *testing.T) {
	validKey := validSecondaryKey()
	require.NoError(t, validKey.Validate())

	require.ErrorIs(t, (SecondaryKey{}).Validate(), ErrMissingSecondaryKey)

	zeroPrivateKey := keys.PrivateKey{Value: big.NewInt(0)}
	zeroValueKey := SecondaryKey{
		SecretKey: &zeroPrivateKey,
		PublicKey: validKey.PublicKey,
	}
	require.ErrorIs(t, zeroValueKey.Validate(), ErrInvalidSecondaryKey)

	otherPrivateKey := keys.NewPrivateKeyFromBytes([32]byte{
		2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2,
	})
	otherPublicKey := otherPrivateKey.ToPublicKey()
	mismatchedKey := SecondaryKey{
		SecretKey: validKey.SecretKey,
		PublicKey: &otherPublicKey,
	}

	err := mismatchedKey.Validate()
	require.True(t, errors.Is(err, ErrInvalidSecondaryKey))
}

func validSecondaryKey() SecondaryKey {
	privateKey := keys.NewPrivateKeyFromBytes([32]byte{
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
	})
	publicKey := privateKey.ToPublicKey()

	return SecondaryKey{
		SecretKey: &privateKey,
		PublicKey: &publicKey,
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
	return poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)
}
