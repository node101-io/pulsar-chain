package abci

import (
	"context"

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
	root []byte
	err  error
}

func (k testBridgeKeeper) GetActionsReducedRoot(context.Context) ([]byte, error) {
	if k.err != nil {
		return nil, k.err
	}
	if k.root != nil {
		return k.root, nil
	}
	return testActionsReducedRoot(), nil
}
