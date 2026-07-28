package types

import (
	"fmt"

	minafield "github.com/node101-io/mina-signer-go/field"
	minasignergo "github.com/node101-io/mina-signer-go/merklelist"
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:                      DefaultParams(),
		BridgeState:                 DefaultBridgeState(),
		ActionsReducedRootSnapshots: DefaultActionsReducedRootSnapshots(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}

	if gs.BridgeState.LatestFetchedMinaHeight < 0 {
		return fmt.Errorf("bridge_state.latest_fetched_mina_height must be non-negative")
	}

	if len(gs.ActionsReducedRootSnapshots) == 0 {
		return fmt.Errorf("actions_reduced_root_snapshots must not be empty")
	}

	var prevHeight int64 = -1

	for i, snapshot := range gs.ActionsReducedRootSnapshots {
		if snapshot.CosmosBlockHeight < 0 {
			return fmt.Errorf("actions_reduced_root_snapshots[%d]: cosmos_block_height must be non-negative", i)
		}
		if len(snapshot.ActionsReducedRoot) == 0 {
			return fmt.Errorf("actions_reduced_root_snapshots[%d]: actions_reduced_root must not be empty", i)
		}
		if _, err := minafield.NewField().FromBytes(snapshot.ActionsReducedRoot); err != nil {
			return fmt.Errorf("actions_reduced_root_snapshots[%d]: actions_reduced_root must be canonical Mina field bytes: %w", i, err)
		}
		if i == 0 && snapshot.CosmosBlockHeight != 0 {
			return fmt.Errorf("actions_reduced_root_snapshots[0]: cosmos_block_height must be 0")
		}
		if i > 0 && snapshot.CosmosBlockHeight <= prevHeight {
			return fmt.Errorf("actions_reduced_root_snapshots must be strictly increasing by cosmos_block_height")
		}

		prevHeight = snapshot.CosmosBlockHeight
	}

	return nil
}

func DefaultBridgeState() BridgeState {
	return BridgeState{
		LatestFetchedMinaHeight: 0,
	}
}

func DefaultActionsReducedRoot() []byte {
	return minasignergo.NewMerkleList(MerkleListPrefix).Root()
}

func DefaultActionsReducedRootSnapshots() []ActionsReducedRootSnapshot {
	return []ActionsReducedRootSnapshot{
		{
			CosmosBlockHeight:  0,
			ActionsReducedRoot: DefaultActionsReducedRoot(),
		},
	}
}
