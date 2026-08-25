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

	rawPublicInputs := make([]byte, 96)
	copy(rawPublicInputs[:32], inputs.SessionPublicKey)
	binary.BigEndian.PutUint64(rawPublicInputs[56:64], inputs.ExpiresAtHeight)
	copy(rawPublicInputs[64:], inputs.Identity)
	expected := sha256.Sum256(rawPublicInputs)

	actual, err := types.ComputePublicInputsHash(inputs)
	require.NoError(t, err)
	require.Equal(t, expected[:], actual)
}

func TestComputePublicInputsHashValidation(t *testing.T) {

	validPublicKeyInputs := validPublicKeyInputs()

	tests := []struct {
		name   string
		inputs *types.PublicKeyInputs
		want   error
	}{
		{
			name: "nil inputs",
			want: types.ErrNilPublicKeyInputs,
		},
		{
			name: "nil public key",
			inputs: &types.PublicKeyInputs{
				ExpiresAtHeight: validPublicKeyInputs.ExpiresAtHeight,
				Identity:        validPublicKeyInputs.Identity,
			},
			want: types.ErrNilPublicKey,
		},
		{
			name: "invalid public key length",
			inputs: &types.PublicKeyInputs{
				SessionPublicKey: validPublicKeyInputs.SessionPublicKey[:types.SessionPublicKeySize-1],
				ExpiresAtHeight:  validPublicKeyInputs.ExpiresAtHeight,
				Identity:         validPublicKeyInputs.Identity,
			},
			want: types.ErrPublicKeyInvalidLength,
		},
		{
			name: "zero expiration height",
			inputs: &types.PublicKeyInputs{
				SessionPublicKey: validPublicKeyInputs.SessionPublicKey,
				ExpiresAtHeight:  0,
				Identity:         validPublicKeyInputs.Identity,
			},
			want: types.ErrInvalidExpirationHeight,
		},
		{
			name: "nil identity",
			inputs: &types.PublicKeyInputs{
				SessionPublicKey: validPublicKeyInputs.SessionPublicKey,
				ExpiresAtHeight:  validPublicKeyInputs.ExpiresAtHeight,
			},
			want: types.ErrNilIdentity,
		},
		{
			name: "invalid identity length",
			inputs: &types.PublicKeyInputs{
				SessionPublicKey: validPublicKeyInputs.SessionPublicKey,
				ExpiresAtHeight:  validPublicKeyInputs.ExpiresAtHeight,
				Identity:         validPublicKeyInputs.Identity[:types.IdentitySize-1],
			},
			want: types.ErrIdentityInvalidLength,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := types.ComputePublicInputsHash(test.inputs)
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
