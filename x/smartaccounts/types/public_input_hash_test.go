package types_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"
)

func TestComputePublicInputsHash(t *testing.T) {
	inputs := validPublicKeyInputs()
	accountAddress := bytes.Repeat([]byte{0x33}, types.AccountAddressSize)

	rawPublicInputs := make([]byte, 128)
	copy(rawPublicInputs[:32], inputs.SessionPublicKey)
	binary.BigEndian.PutUint64(rawPublicInputs[56:64], inputs.ExpiresAtHeight)
	copy(rawPublicInputs[64:96], inputs.Identity)
	copy(rawPublicInputs[128-types.AccountAddressSize:], accountAddress)
	expected := sha256.Sum256(rawPublicInputs)

	actual, err := types.ComputePublicInputsHash(inputs, accountAddress)
	require.NoError(t, err)
	require.Equal(t, expected[:], actual)
}

func TestComputePublicInputsHashValidation(t *testing.T) {

	validPublicKeyInputs := validPublicKeyInputs()
	validAccountAddress := bytes.Repeat([]byte{0x33}, types.AccountAddressSize)

	tests := []struct {
		name           string
		inputs         *types.PublicKeyInputs
		accountAddress []byte
		want           error
	}{
		{
			name:           "nil inputs",
			accountAddress: validAccountAddress,
			want:           types.ErrNilPublicKeyInputs,
		},
		{
			name: "nil public key",
			inputs: &types.PublicKeyInputs{
				ExpiresAtHeight: validPublicKeyInputs.ExpiresAtHeight,
				Identity:        validPublicKeyInputs.Identity,
			},
			accountAddress: validAccountAddress,
			want:           types.ErrNilPublicKey,
		},
		{
			name: "invalid public key length",
			inputs: &types.PublicKeyInputs{
				SessionPublicKey: validPublicKeyInputs.SessionPublicKey[:types.SessionPublicKeySize-1],
				ExpiresAtHeight:  validPublicKeyInputs.ExpiresAtHeight,
				Identity:         validPublicKeyInputs.Identity,
			},
			accountAddress: validAccountAddress,
			want:           types.ErrPublicKeyInvalidLength,
		},
		{
			name: "zero expiration height",
			inputs: &types.PublicKeyInputs{
				SessionPublicKey: validPublicKeyInputs.SessionPublicKey,
				ExpiresAtHeight:  0,
				Identity:         validPublicKeyInputs.Identity,
			},
			accountAddress: validAccountAddress,
			want:           types.ErrInvalidExpirationHeight,
		},
		{
			name: "nil identity",
			inputs: &types.PublicKeyInputs{
				SessionPublicKey: validPublicKeyInputs.SessionPublicKey,
				ExpiresAtHeight:  validPublicKeyInputs.ExpiresAtHeight,
			},
			accountAddress: validAccountAddress,
			want:           types.ErrNilIdentity,
		},
		{
			name: "invalid identity length",
			inputs: &types.PublicKeyInputs{
				SessionPublicKey: validPublicKeyInputs.SessionPublicKey,
				ExpiresAtHeight:  validPublicKeyInputs.ExpiresAtHeight,
				Identity:         validPublicKeyInputs.Identity[:types.IdentitySize-1],
			},
			accountAddress: validAccountAddress,
			want:           types.ErrIdentityInvalidLength,
		},
		{
			name:           "nil account address",
			inputs:         validPublicKeyInputs,
			accountAddress: nil,
			want:           types.ErrNilAccountAddress,
		},
		{
			name:           "invalid account address length",
			inputs:         validPublicKeyInputs,
			accountAddress: validAccountAddress[:types.AccountAddressSize-1],
			want:           types.ErrInvalidAccountAddress,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := types.ComputePublicInputsHash(test.inputs, test.accountAddress)
			require.ErrorIs(t, err, test.want)
		})
	}
}

func validPublicKeyInputs() *types.PublicKeyInputs {
	return &types.PublicKeyInputs{
		SessionPublicKey: bytes.Repeat([]byte{0x11}, types.SessionPublicKeySize),
		ExpiresAtHeight:  42,
		Identity:         bytes.Repeat([]byte{0x22}, types.IdentitySize),
	}
}
