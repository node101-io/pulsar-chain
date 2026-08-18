package abci

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	minafield "github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/mina-signer-go/privatekey"
	votepersistence "github.com/node101-io/pulsar-chain/x/votepersistence/types"
	"github.com/stretchr/testify/require"
)

type fieldVerifierValidator struct {
	secondaryKey SecondaryKey
	power        int64
}

type voteExtBodyVector struct {
	AppHashBase64            string `json:"appHashBase64"`
	BlockHeight              int64  `json:"blockHeight"`
	ValidatorSetRootDecimal  string `json:"validatorSetRootDecimal"`
	ActionsReducedRootBase64 string `json:"actionsReducedRootBase64"`
	StateRootDecimal         string `json:"stateRootDecimal"`
	BodyHashDecimal          string `json:"bodyHashDecimal"`
}

func loadVoteExtBodyVector(t *testing.T) voteExtBodyVector {
	t.Helper()

	path := filepath.Join("..", "scripts", "vote-ext-verifier", "vote-ext-body-vector.json")
	bz, err := os.ReadFile(path)
	require.NoError(t, err)

	var vector voteExtBodyVector
	require.NoError(t, json.Unmarshal(bz, &vector))
	return vector
}

func TestVoteExtSignatureMatchesFieldVerifierVector(t *testing.T) {
	vector := loadVoteExtBodyVector(t)
	validators := []fieldVerifierValidator{
		{secondaryKey: secondaryKeyFromScalarUint64(t, 1), power: 100},
		{secondaryKey: secondaryKeyFromScalarUint64(t, 2), power: 101},
		{secondaryKey: secondaryKeyFromScalarUint64(t, 3), power: 102},
	}

	validatorSetRoot := validatorSetRootForFieldVerifier(t, validators)
	require.NotNil(t, validatorSetRoot)

	appHash, err := base64.StdEncoding.DecodeString(vector.AppHashBase64)
	require.NoError(t, err)
	require.NotNil(t, appHash)
	actionsReducedRoot, err := base64.StdEncoding.DecodeString(vector.ActionsReducedRootBase64)
	require.NoError(t, err)
	require.Equal(t, testActionsReducedRoot(), actionsReducedRoot)

	body := votepersistence.VoteExtBody{
		NextValidatorSetHash: validatorSetRoot.Bytes(),
		CurrentStateRoot:     appHash,
		CurrentBlockHeight:   vector.BlockHeight,
		ActionsReducedRoot:   actionsReducedRoot,
	}

	proofCommitment := testProofCommitment()
	signature, err := validators[0].secondaryKey.SignVoteExtension(body, proofCommitment)
	require.NoError(t, err)
	require.NotEmpty(t, signature)

	poseidonHash := testPoseidonHash()
	require.NotNil(t, poseidonHash)

	err = verifyVoteExtSig(
		poseidonHash,
		signature,
		body,
		proofCommitment,
		validators[0].secondaryKey.PublicKey.Bytes(),
		NetworkID,
	)
	require.NoError(t, err)

	stateRoot, err := encodeVoteExtBodyForHash(poseidonHash, appHash)
	require.NoError(t, err)

	bodyHash, err := hashVoteExtBody(poseidonHash, body)
	require.NoError(t, err)
	require.NotNil(t, bodyHash)

	require.Equal(t, vector.ValidatorSetRootDecimal, validatorSetRoot.String())
	require.Equal(t, vector.StateRootDecimal, stateRoot.String())
	require.Equal(t, vector.BodyHashDecimal, bodyHash.String())
}

func secondaryKeyFromScalarUint64(t *testing.T, scalar uint64) SecondaryKey {
	t.Helper()

	var skBytes [32]byte
	binary.BigEndian.PutUint64(skBytes[24:], scalar)

	secretKey, err := privatekey.NewPrivateKeyFromBytes(skBytes, NetworkID)
	require.NoError(t, err)

	publicKey, err := secretKey.ToPublicKey()
	require.NoError(t, err)

	return SecondaryKey{
		SecretKey: secretKey,
		PublicKey: publicKey,
	}
}

func validatorSetRootForFieldVerifier(
	t *testing.T,
	validators []fieldVerifierValidator,
) *minafield.FieldElement {
	t.Helper()

	field := minafield.NewField()
	poseidonHash := poseidon.NewPoseidon()

	sort.Slice(validators, func(i, j int) bool {
		return validators[i].power < validators[j].power
	})

	acc, err := poseidonHash.HashFieldElements(field.Zero())
	require.NoError(t, err)

	for _, validator := range validators {
		x, isOdd, err := validator.secondaryKey.PublicKey.ToFields()
		require.NoError(t, err)

		leaf, err := poseidonHash.HashFieldElementsWithPrefix(
			ValidatorSetEntryHashPrefix,
			x,
			isOdd,
			field.FromUint64(uint64(validator.power)),
		)
		require.NoError(t, err)

		acc, err = poseidonHash.HashFieldElements(acc, leaf)
		require.NoError(t, err)
	}

	return acc
}
