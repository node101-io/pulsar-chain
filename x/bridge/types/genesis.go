package types

import minasignergo "github.com/node101-io/mina-signer-go/merklelist"

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:      DefaultParams(),
		BridgeState: DefaultBridgeState(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	return gs.Params.Validate()
}

func DefaultBridgeState() BridgeState {

	return BridgeState{
		LatestFetchedMinaHeight: 0,
		ActionsReducedRoot:      minasignergo.NewMerkleList(MerkleListPrefix).Root(),
	}
}
