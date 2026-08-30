package keeper

import "github.com/node101-io/pulsar-chain/x/verification/types"

// queryServer exposes deterministic consensus state such as registered proofs,
// commitments, votes, tallies, and final results. It does not proxy sidecar
// status because QUEUED, VERIFYING, or FAILED may differ between validators and
// therefore cannot be presented as shared chain state.
type queryServer struct {
	k Keeper
}

// NewQueryServerImpl creates the verification Query service.
func NewQueryServerImpl(k Keeper) types.QueryServer {
	return &queryServer{k: k}
}

var _ types.QueryServer = (*queryServer)(nil)
