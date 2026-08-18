package keeper

import (
	"cosmossdk.io/collections"
	collectionscodec "cosmossdk.io/collections/codec"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

type proofIDKeyCodec struct {
	pairCodec collectionscodec.KeyCodec[collections.Pair[int64, int64]]
}

func newProofIDKeyCodec() collectionscodec.KeyCodec[types.ProofID] {
	return proofIDKeyCodec{
		pairCodec: collections.NamedPairKeyCodec(
			"block_height",
			collections.Int64Key,
			"proof_index",
			collections.Int64Key,
		),
	}
}

func (c proofIDKeyCodec) Encode(buffer []byte, key types.ProofID) (int, error) {
	return c.pairCodec.Encode(buffer, proofIDPair(key))
}

func (c proofIDKeyCodec) Decode(buffer []byte) (int, types.ProofID, error) {
	read, pair, err := c.pairCodec.Decode(buffer)
	if err != nil {
		return 0, types.ProofID{}, err
	}

	return read, types.ProofID{
		BlockHeight: pair.K1(),
		ProofIndex:  pair.K2(),
	}, nil
}

func (c proofIDKeyCodec) Size(key types.ProofID) int {
	return c.pairCodec.Size(proofIDPair(key))
}

func (c proofIDKeyCodec) EncodeJSON(key types.ProofID) ([]byte, error) {
	return c.pairCodec.EncodeJSON(proofIDPair(key))
}

func (c proofIDKeyCodec) DecodeJSON(b []byte) (types.ProofID, error) {
	pair, err := c.pairCodec.DecodeJSON(b)
	if err != nil {
		return types.ProofID{}, err
	}

	return types.ProofID{
		BlockHeight: pair.K1(),
		ProofIndex:  pair.K2(),
	}, nil
}

func (c proofIDKeyCodec) Stringify(key types.ProofID) string {
	return c.pairCodec.Stringify(proofIDPair(key))
}

func (proofIDKeyCodec) KeyType() string {
	return "pulsarchain.verification.v1.ProofID"
}

func (c proofIDKeyCodec) EncodeNonTerminal(buffer []byte, key types.ProofID) (int, error) {
	return c.pairCodec.EncodeNonTerminal(buffer, proofIDPair(key))
}

func (c proofIDKeyCodec) DecodeNonTerminal(buffer []byte) (int, types.ProofID, error) {
	read, pair, err := c.pairCodec.DecodeNonTerminal(buffer)
	if err != nil {
		return 0, types.ProofID{}, err
	}

	return read, types.ProofID{
		BlockHeight: pair.K1(),
		ProofIndex:  pair.K2(),
	}, nil
}

func (c proofIDKeyCodec) SizeNonTerminal(key types.ProofID) int {
	return c.pairCodec.SizeNonTerminal(proofIDPair(key))
}

func proofIDPair(key types.ProofID) collections.Pair[int64, int64] {
	return collections.Join(key.BlockHeight, key.ProofIndex)
}
