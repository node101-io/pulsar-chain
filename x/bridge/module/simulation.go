package bridge

import (
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	params := types.DefaultTestParams()

	bridgeGenesis := types.GenesisState{
		Params:                      params,
		BridgeState:                 types.NewInitialBridgeState(params.StartBlockHeight),
		ActionsReducedRootSnapshots: types.DefaultActionsReducedRootSnapshots(),
	}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&bridgeGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {}

// WeightedOperations returns the all the gov module operations with their respective weights.
func (AppModule) WeightedOperations(module.SimulationState) []simtypes.WeightedOperation {
	return nil
}

func (AppModule) ProposalMsgs(module.SimulationState) []simtypes.WeightedProposalMsg {
	return nil
}
