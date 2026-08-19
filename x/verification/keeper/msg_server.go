package keeper

import "github.com/node101-io/pulsar-chain/x/verification/types"

// msgServer exposes public transaction handlers for proof registration and
// governance parameters. Commitments and revelations are validator consensus
// actions, so they arrive through authenticated vote extensions rather than
// user-broadcast transactions that anyone could place in the mempool.
type msgServer struct {
	Keeper
}

// NewMsgServerImpl creates the verification Msg service.
func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{Keeper: k}
}

var _ types.MsgServer = (*msgServer)(nil)
