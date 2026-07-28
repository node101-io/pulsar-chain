package abci

import (
	"context"
	"fmt"

	minafield "github.com/node101-io/mina-signer-go/field"
)

func mustReduceToFieldBytes(raw []byte) []byte {
	root, err := minafield.NewField().FromBytesBEReduce(raw)
	if err != nil {
		panic(err)
	}
	return root.Bytes()
}

func testActionsReducedRoot() []byte {
	return mustReduceToFieldBytes([]byte("pulsar"))
}

func wrongTestActionsReducedRoot() []byte {
	return mustReduceToFieldBytes([]byte("wrong-root"))
}

type testBridgeKeeper struct {
	rootsByHeight map[int64][]byte
	err           error
}

func (k testBridgeKeeper) GetActionsReducedRootAtHeight(_ context.Context, height int64) ([]byte, error) {
	if k.err != nil {
		return nil, k.err
	}

	if len(k.rootsByHeight) == 0 {
		return testActionsReducedRoot(), nil
	}

	var bestHeight int64
	var bestRoot []byte
	found := false

	for h, root := range k.rootsByHeight {
		if h <= height && (!found || h > bestHeight) {
			bestHeight = h
			bestRoot = root
			found = true
		}
	}

	if !found {
		return nil, fmt.Errorf("no actions reduced root snapshot at or before height %d", height)
	}

	return bestRoot, nil
}
