package keeper

import "github.com/node101-io/pulsar-chain/x/verification/types"

type msgServer struct {
	Keeper
}

func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{Keeper: k}
}

var _ types.MsgServer = (*msgServer)(nil)
