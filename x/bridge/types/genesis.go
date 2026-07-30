package types

import (
	"bytes"

	errorsmod "cosmossdk.io/errors"
	minafield "github.com/node101-io/mina-signer-go/field"
	minasignergo "github.com/node101-io/mina-signer-go/merklelist"
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	params := DefaultParams()

	return &GenesisState{
		Params:                      params,
		BridgeState:                 NewInitialBridgeState(params.StartBlockHeight),
		ActionsReducedRootSnapshots: DefaultActionsReducedRootSnapshots(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}

	if err := gs.BridgeState.Validate(); err != nil {
		return err
	}

	minLatestFetched := gs.Params.StartBlockHeight - 1
	if gs.BridgeState.LatestFetchedMinaHeight < minLatestFetched {
		return errorsmod.Wrapf(
			ErrLatestFetchedMinaHeightBeforeStartBlock,
			"latest_fetched_mina_height %d must be >= start_block_height - 1 (%d)",
			gs.BridgeState.LatestFetchedMinaHeight,
			minLatestFetched,
		)
	}

	return validateActionsReducedRootSnapshots(gs.ActionsReducedRootSnapshots)
}

func DefaultBridgeState() BridgeState {
	return NewInitialBridgeState(DefaultParams().StartBlockHeight)
}

func NewInitialBridgeState(startBlockHeight int64) BridgeState {
	return BridgeState{
		LatestFetchedMinaHeight: startBlockHeight - 1,
	}
}

func DefaultActionsReducedRoot() []byte {
	return minasignergo.NewMerkleList(ActionsReducedRootMerkleListPrefixV1).Root()
}

func DefaultActionsReducedRootSnapshots() []ActionsReducedRootSnapshot {
	return []ActionsReducedRootSnapshot{
		{
			CosmosBlockHeight:  0,
			ActionsReducedRoot: DefaultActionsReducedRoot(),
		},
	}
}
func validateActionsReducedRootSnapshots(snapshots []ActionsReducedRootSnapshot) error {
	if len(snapshots) == 0 {
		return ErrEmptyActionsReducedRootSnapshots
	}

	fieldCodec := minafield.NewField()
	expectedRootLen := fieldCodec.ElementSize()
	defaultRoot := DefaultActionsReducedRoot()

	var prevHeight int64 = -1

	for i, snapshot := range snapshots {
		if snapshot.CosmosBlockHeight < 0 {
			return errorsmod.Wrapf(
				ErrInvalidActionsReducedRootSnapshotHeight,
				"actions_reduced_root_snapshots[%d]",
				i,
			)
		}

		if len(snapshot.ActionsReducedRoot) != expectedRootLen {
			return errorsmod.Wrapf(
				ErrInvalidActionsReducedRoot,
				"actions_reduced_root_snapshots[%d]: expected %d bytes, got %d",
				i,
				expectedRootLen,
				len(snapshot.ActionsReducedRoot),
			)
		}

		rootElement, err := fieldCodec.FromBytes(snapshot.ActionsReducedRoot)
		if err != nil {
			return errorsmod.Wrapf(
				ErrInvalidActionsReducedRoot,
				"actions_reduced_root_snapshots[%d]: %v",
				i,
				err,
			)
		}

		if !bytes.Equal(rootElement.Bytes(), snapshot.ActionsReducedRoot) {
			return errorsmod.Wrapf(
				ErrInvalidActionsReducedRoot,
				"actions_reduced_root_snapshots[%d]: non-canonical field bytes",
				i,
			)
		}

		if i == 0 {
			if snapshot.CosmosBlockHeight != 0 {
				return errorsmod.Wrapf(
					ErrActionsReducedRootSnapshotsMustStartAtZero,
					"got %d",
					snapshot.CosmosBlockHeight,
				)
			}

			if !bytes.Equal(snapshot.ActionsReducedRoot, defaultRoot) {
				return ErrInvalidInitialActionsReducedRoot
			}
		}

		if i > 0 && snapshot.CosmosBlockHeight <= prevHeight {
			return errorsmod.Wrapf(
				ErrActionsReducedRootSnapshotsMustBeIncreasing,
				"actions_reduced_root_snapshots[%d]: %d <= %d",
				i,
				snapshot.CosmosBlockHeight,
				prevHeight,
			)
		}

		prevHeight = snapshot.CosmosBlockHeight
	}

	return nil
}
