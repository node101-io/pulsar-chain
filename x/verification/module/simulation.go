package verification

import (
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

// GenerateGenesisState adds an empty verification state with production bounds
// to SDK simulations. Active protocol records are created only by real module
// transitions, so simulation genesis does not invent proofs or commitments.
func (AppModule) GenerateGenesisState(state *module.SimulationState) {
	genesis := types.GenesisState{Params: types.DefaultParams()}
	state.GenState[types.ModuleName] = state.Cdc.MustMarshalJSON(&genesis)
}

// RegisterStoreDecoder has no custom simulation decoder.
func (AppModule) RegisterStoreDecoder(simtypes.StoreDecoderRegistry) {}

// WeightedOperations returns no randomized verification transactions. Proof
// bytes and sidecar completion are external to this module, and validator-only
// commitments cannot be modeled as ordinary user operations.
func (AppModule) WeightedOperations(module.SimulationState) []simtypes.WeightedOperation {
	return nil
}

// ProposalMsgs returns no randomized governance proposals because this module
// currently adds no custom proposal type beyond the standard parameter update.
func (AppModule) ProposalMsgs(module.SimulationState) []simtypes.WeightedProposalMsg {
	return nil
}
