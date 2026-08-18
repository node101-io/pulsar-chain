package verification

import (
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func (AppModule) GenerateGenesisState(state *module.SimulationState) {
	genesis := types.GenesisState{Params: types.DefaultParams()}
	state.GenState[types.ModuleName] = state.Cdc.MustMarshalJSON(&genesis)
}

func (AppModule) RegisterStoreDecoder(simtypes.StoreDecoderRegistry) {}

func (AppModule) WeightedOperations(module.SimulationState) []simtypes.WeightedOperation {
	return nil
}

func (AppModule) ProposalMsgs(module.SimulationState) []simtypes.WeightedProposalMsg {
	return nil
}
