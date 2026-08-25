package keeper_test

import (
	"bytes"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"
)

func TestAppendSessionKeyToSmartAccount(t *testing.T) {
	f := initFixture(t)
	identity := bytes.Repeat([]byte{0x01}, types.IdentitySize)
	key := types.SessionKey{
		PublicKey:       bytes.Repeat([]byte{0x02}, types.SessionPublicKeySize),
		ExpiresAtHeight: 10,
	}

	exists, err := f.keeper.HasSmartAccount(f.ctx, identity)
	require.NoError(t, err)
	require.False(t, exists)

	require.NoError(t, f.keeper.AppendSessionKeyToSmartAccount(f.ctx, identity, key))

	exists, err = f.keeper.HasSmartAccount(f.ctx, identity)
	require.NoError(t, err)
	require.True(t, exists)

	err = f.keeper.AppendSessionKeyToSmartAccount(f.ctx, identity, key)
	require.ErrorIs(t, err, types.ErrSessionKeyAlreadyExists)
}

func TestAppendSessionKeyPrunesExpiredKeys(t *testing.T) {
	f := initFixture(t)
	identity := bytes.Repeat([]byte{0x01}, types.IdentitySize)
	expiringKey := types.SessionKey{
		PublicKey:       bytes.Repeat([]byte{0x02}, types.SessionPublicKeySize),
		ExpiresAtHeight: 2,
	}

	f.ctx = sdk.UnwrapSDKContext(f.ctx).WithBlockHeight(1)
	require.NoError(t, f.keeper.AppendSessionKeyToSmartAccount(f.ctx, identity, expiringKey))

	f.ctx = sdk.UnwrapSDKContext(f.ctx).WithBlockHeight(2)
	activeKey := types.SessionKey{
		PublicKey:       bytes.Repeat([]byte{0x03}, types.SessionPublicKeySize),
		ExpiresAtHeight: 3,
	}
	require.NoError(t, f.keeper.AppendSessionKeyToSmartAccount(f.ctx, identity, activeKey))

	// The first key can be appended again because the previous copy was pruned.
	require.NoError(t, f.keeper.AppendSessionKeyToSmartAccount(f.ctx, identity, expiringKey))
}

func TestHasSmartAccountValidation(t *testing.T) {
	f := initFixture(t)

	tests := []struct {
		name     string
		identity []byte
		want     error
	}{
		{name: "nil identity", identity: nil, want: types.ErrNilIdentity},
		{name: "empty identity", identity: []byte{}, want: types.ErrNilIdentity},
		{name: "invalid identity length", identity: []byte{0x01}, want: types.ErrIdentityInvalidLength},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := f.keeper.HasSmartAccount(f.ctx, test.identity)
			require.ErrorIs(t, err, test.want)
		})
	}
}
