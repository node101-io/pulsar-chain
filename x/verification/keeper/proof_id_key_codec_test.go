package keeper

import (
	"testing"

	"cosmossdk.io/collections"
	"cosmossdk.io/collections/colltest"
	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func TestProofIDKeyCodec(t *testing.T) {
	key := types.ProofID{
		BlockHeight: 42,
		ProofIndex:  7,
	}

	colltest.TestKeyCodec(t, newProofIDKeyCodec(), key)
}

func TestProofIDKeyCodecPreservesPairEncoding(t *testing.T) {
	key := types.ProofID{
		BlockHeight: 42,
		ProofIndex:  7,
	}
	proofIDCodec := newProofIDKeyCodec()
	pairCodec := collections.PairKeyCodec(collections.Int64Key, collections.Int64Key)

	proofIDBytes := make([]byte, proofIDCodec.Size(key))
	written, err := proofIDCodec.Encode(proofIDBytes, key)
	require.NoError(t, err)
	require.Equal(t, len(proofIDBytes), written)

	pairKey := collections.Join(key.BlockHeight, key.ProofIndex)
	pairBytes := make([]byte, pairCodec.Size(pairKey))
	written, err = pairCodec.Encode(pairBytes, pairKey)
	require.NoError(t, err)
	require.Equal(t, len(pairBytes), written)

	require.Equal(t, pairBytes, proofIDBytes)
}
