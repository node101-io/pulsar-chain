package types

import (
	"bytes"
	"encoding/json"

	errorsmod "cosmossdk.io/errors"
	minafield "github.com/node101-io/mina-signer-go/field"
	minasignergo "github.com/node101-io/mina-signer-go/merklelist"
)

type bridgeStateJSON struct {
	LatestFetchedMinaHeight            int64    `json:"latest_fetched_mina_height,omitempty"`
	ValidActionHashes                  []string `json:"valid_action_hashes"`
	ValidActionHashesCosmosBlockHeight int64    `json:"valid_action_hashes_cosmos_block_height,omitempty"`
}

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:                      DefaultParams(),
		BridgeState:                 DefaultTestBridgeState(),
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

	// Genesis carries the recent consensus window; its newest snapshot is current.
	return validateActionsReducedRootSnapshots(
		gs.ActionsReducedRootSnapshots,
		gs.Params.ActionsReducedRootSnapshotWindowSize,
	)
}

// DefaultBridgeState returns the default bridge state.
func DefaultBridgeState() BridgeState {
	return BridgeState{
		ValidActionHashes: []string{},
	}
}

// MarshalJSON keeps empty action hashes stable as [] instead of null/omitted.
func (s BridgeState) MarshalJSON() ([]byte, error) {
	hashes := s.ValidActionHashes
	if hashes == nil {
		hashes = []string{}
	}

	return json.Marshal(bridgeStateJSON{
		LatestFetchedMinaHeight:            s.LatestFetchedMinaHeight,
		ValidActionHashes:                  hashes,
		ValidActionHashesCosmosBlockHeight: s.ValidActionHashesCosmosBlockHeight,
	})
}

// UnmarshalJSON normalizes omitted action hashes back to an empty slice.
func (s *BridgeState) UnmarshalJSON(bz []byte) error {
	var decoded bridgeStateJSON
	if err := json.Unmarshal(bz, &decoded); err != nil {
		return err
	}

	s.LatestFetchedMinaHeight = decoded.LatestFetchedMinaHeight
	s.ValidActionHashes = decoded.ValidActionHashes
	s.ValidActionHashesCosmosBlockHeight = decoded.ValidActionHashesCosmosBlockHeight
	if s.ValidActionHashes == nil {
		s.ValidActionHashes = []string{}
	}

	return nil
}

// DefaultTestBridgeState returns the initial bridge state for tests and simulation.
func DefaultTestBridgeState() BridgeState {
	return BridgeState{
		LatestFetchedMinaHeight: defaultStartBlockHeight - 1,
		ValidActionHashes:       []string{},
	}
}

// NewInitialBridgeState initializes the bridge cursor so the first query starts
// exactly at startBlockHeight.
func NewInitialBridgeState(startBlockHeight int64) BridgeState {
	return BridgeState{
		LatestFetchedMinaHeight: startBlockHeight - 1,
		ValidActionHashes:       []string{},
	}
}

// DefaultActionsReducedRoot returns the protocol-defined empty root for the
// versioned actions-reduced-root Merkle list.
func DefaultActionsReducedRoot() []byte {
	return minasignergo.NewMerkleList(ActionsReducedRootMerkleListPrefixV1).Root()
}

// DefaultActionsReducedRootSnapshots returns the required initial snapshot set:
// cosmos block height 0 with the empty actions reduced root.
func DefaultActionsReducedRootSnapshots() []ActionsReducedRootSnapshot {
	return []ActionsReducedRootSnapshot{
		{
			CosmosBlockHeight:  0,
			ActionsReducedRoot: DefaultActionsReducedRoot(),
		},
	}
}
func validateActionsReducedRootSnapshots(
	snapshots []ActionsReducedRootSnapshot,
	windowSize int64,
) error {
	if len(snapshots) == 0 {
		return ErrEmptyActionsReducedRootSnapshots
	}

	if int64(len(snapshots)) > windowSize {
		return errorsmod.Wrapf(
			ErrTooManyActionsReducedRootSnapshots,
			"got %d, max %d",
			len(snapshots),
			windowSize,
		)
	}

	var prevHeight int64 = -1

	for i, snapshot := range snapshots {
		if snapshot.CosmosBlockHeight < 0 {
			return errorsmod.Wrapf(
				ErrInvalidActionsReducedRootSnapshotHeight,
				"actions_reduced_root_snapshots[%d]",
				i,
			)
		}

		if err := validateActionsReducedRoot(snapshot.ActionsReducedRoot); err != nil {
			return err
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

func validateActionsReducedRoot(root []byte) error {
	fieldCodec := minafield.NewField()

	if len(root) != fieldCodec.ElementSize() {
		return errorsmod.Wrapf(
			ErrInvalidActionsReducedRoot,
			"expected %d bytes, got %d",
			fieldCodec.ElementSize(),
			len(root),
		)
	}

	rootElement, err := fieldCodec.FromBytes(root)
	if err != nil {
		return errorsmod.Wrap(ErrInvalidActionsReducedRoot, err.Error())
	}

	if !bytes.Equal(rootElement.Bytes(), root) {
		return errorsmod.Wrap(ErrInvalidActionsReducedRoot, "non-canonical field bytes")
	}

	return nil
}
