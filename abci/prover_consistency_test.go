package abci

import (
	"encoding/base64"
	"encoding/binary"
	"sort"
	"testing"

	"github.com/node101-io/mina-signer-go/field"
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

const (
	preComputedValidatorSetRoot = "28268229887385077809518158266483127416536537714268800272951471493872741302582"
	preComputedStateRoot        = "13804078167964076827860295044345828310151277215339982145024802239006146310463"
	preComputedVoteExtBodyHash  = "27597929105583874691902559413948775124877758774604548537116974868579425858141"
)

const (
	appHashB64  = "kLjv6/1CeuJ6aGKQYXusYfxtZlN2iKIzIwi7oEnOcPA="
	blockHeight = 176770
)

func TestVoteExtSignatureMatchesFieldVerifierVector(t *testing.T) {
	validators := []fieldVerifierValidator{
		{secondaryKey: secondaryKeyFromScalarUint64(t, 1), power: 100},
		{secondaryKey: secondaryKeyFromScalarUint64(t, 2), power: 101},
		{secondaryKey: secondaryKeyFromScalarUint64(t, 3), power: 102},
	}

	validatorSetRoot := validatorSetRootForFieldVerifier(t, validators)
	require.NotNil(t, validatorSetRoot)

	appHash, err := base64.StdEncoding.DecodeString(appHashB64)
	require.NoError(t, err)
	require.NotNil(t, appHash)

	body := votepersistence.VoteExtBody{
		NextValidatorSetHash: validatorSetRoot.Bytes(),
		CurrentStateRoot:     appHash,
		CurrentBlockHeight:   blockHeight,
		ActionsReducedRoot:   ActionsReducedRoot,
	}

	signature, err := validators[0].secondaryKey.SignVoteExtBody(body)
	require.NoError(t, err)
	require.NotEmpty(t, signature)

	poseidonHash := testPoseidonHash()
	require.NotNil(t, poseidonHash)

	err = verifyVoteExtSig(
		poseidonHash,
		signature,
		body,
		validators[0].secondaryKey.PublicKey.Bytes(),
		ActionsReducedRoot,
		NetworkID,
	)
	require.NoError(t, err)

	minaField := field.NewField()
	require.NotNil(t, minaField)

	stateRoot, err := encodeVoteExtBodyForHash(poseidonHash, minafield.NewField(), appHash)
	require.NoError(t, err)

	bodyHash, err := hashVoteExtBody(minaField, poseidonHash, body)
	require.NoError(t, err)
	require.NotNil(t, bodyHash)

	require.Equal(t, validatorSetRoot.String(), preComputedValidatorSetRoot)
	require.Equal(t, stateRoot.String(), preComputedStateRoot)
	require.Equal(t, bodyHash.String(), preComputedVoteExtBodyHash)
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
