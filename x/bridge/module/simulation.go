package bridge

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	bridgesimulation "github.com/node101-io/pulsar-chain/x/bridge/simulation"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	accs := make([]string, len(simState.Accounts))
	for i, acc := range simState.Accounts {
		accs[i] = acc.Address.String()
	}
	bridgeGenesis := types.GenesisState{
		Params: types.DefaultParams(),
	}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&bridgeGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {}

// WeightedOperations returns the all the gov module operations with their respective weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	operations := make([]simtypes.WeightedOperation, 0)
	const (
		opWeightMsgPushNewActions          = "op_weight_msg_bridge"
		defaultWeightMsgPushNewActions int = 100
	)

	var weightMsgPushNewActions int
	simState.AppParams.GetOrGenerate(opWeightMsgPushNewActions, &weightMsgPushNewActions, nil,
		func(_ *rand.Rand) {
			weightMsgPushNewActions = defaultWeightMsgPushNewActions
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgPushNewActions,
		bridgesimulation.SimulateMsgPushNewActions(am.authKeeper, am.bankKeeper, am.keeper, simState.TxConfig),
	))

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(simState module.SimulationState) []simtypes.WeightedProposalMsg {
	return []simtypes.WeightedProposalMsg{}
}
