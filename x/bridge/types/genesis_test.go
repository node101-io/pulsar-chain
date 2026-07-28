package types_test

import (
	"bytes"
	"encoding/json"
	"testing"

	minafield "github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
	"github.com/stretchr/testify/require"
)

func validGenesisState() *types.GenesisState {
	return types.DefaultGenesis()
}

func canonicalRoot(v uint64) []byte {
	return minafield.NewField().FromUint64(v).Bytes()
}

func nonCanonicalRoot() []byte {
	return bytes.Repeat([]byte{0xff}, minafield.NewField().ElementSize())
}

func TestDefaultGenesisValidate(t *testing.T) {
	require.NoError(t, types.DefaultGenesis().Validate())
}

func TestGenesisStateValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*types.GenesisState)
		wantErr error
	}{
		{
			name:   "valid genesis state",
			mutate: func(gs *types.GenesisState) {},
		},
		{
			name: "negative latest fetched mina height",
			mutate: func(gs *types.GenesisState) {
				gs.BridgeState.LatestFetchedMinaHeight = -1
			},
			wantErr: types.ErrInvalidLatestFetchedMinaHeight,
		},
		{
			name: "empty snapshots",
			mutate: func(gs *types.GenesisState) {
				gs.ActionsReducedRootSnapshots = nil
			},
			wantErr: types.ErrEmptyActionsReducedRootSnapshots,
		},
		{
			name: "first snapshot height must be zero",
			mutate: func(gs *types.GenesisState) {
				gs.ActionsReducedRootSnapshots[0].CosmosBlockHeight = 1
			},
			wantErr: types.ErrActionsReducedRootSnapshotsMustStartAtZero,
		},
		{
			name: "first snapshot root must equal default root",
			mutate: func(gs *types.GenesisState) {
				gs.ActionsReducedRootSnapshots[0].ActionsReducedRoot = canonicalRoot(42)
			},
			wantErr: types.ErrInvalidInitialActionsReducedRoot,
		},
		{
			name: "nil root is invalid",
			mutate: func(gs *types.GenesisState) {
				gs.ActionsReducedRootSnapshots[0].ActionsReducedRoot = nil
			},
			wantErr: types.ErrInvalidActionsReducedRoot,
		},
		{
			name: "wrong root length is invalid",
			mutate: func(gs *types.GenesisState) {
				gs.ActionsReducedRootSnapshots[0].ActionsReducedRoot = []byte{1, 2, 3}
			},
			wantErr: types.ErrInvalidActionsReducedRoot,
		},
		{
			name: "non canonical root is invalid",
			mutate: func(gs *types.GenesisState) {
				gs.ActionsReducedRootSnapshots = append(
					gs.ActionsReducedRootSnapshots,
					types.ActionsReducedRootSnapshot{
						CosmosBlockHeight:  10,
						ActionsReducedRoot: nonCanonicalRoot(),
					},
				)
			},
			wantErr: types.ErrInvalidActionsReducedRoot,
		},
		{
			name: "snapshot heights must be strictly increasing",
			mutate: func(gs *types.GenesisState) {
				gs.ActionsReducedRootSnapshots = append(
					gs.ActionsReducedRootSnapshots,
					types.ActionsReducedRootSnapshot{
						CosmosBlockHeight:  0,
						ActionsReducedRoot: canonicalRoot(42),
					},
				)
			},
			wantErr: types.ErrActionsReducedRootSnapshotsMustBeIncreasing,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gs := validGenesisState()
			tc.mutate(gs)

			err := gs.Validate()
			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}

			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestGenesisStateValidateAcceptsCustomCanonicalSnapshotRoot(t *testing.T) {
	gs := validGenesisState()
	gs.ActionsReducedRootSnapshots = append(
		gs.ActionsReducedRootSnapshots,
		types.ActionsReducedRootSnapshot{
			CosmosBlockHeight:  10,
			ActionsReducedRoot: canonicalRoot(42),
		},
	)

	require.NoError(t, gs.Validate())
}

func TestDefaultGenesisJSONRoundTrip(t *testing.T) {
	gs := types.DefaultGenesis()

	bz, err := json.Marshal(gs)
	require.NoError(t, err)

	var roundTripped types.GenesisState
	require.NoError(t, json.Unmarshal(bz, &roundTripped))
	require.Equal(t, *gs, roundTripped)
	require.NoError(t, roundTripped.Validate())
}

func TestGenesisStateJSONRoundTripWithCustomCanonicalSnapshotRoot(t *testing.T) {
	gs := validGenesisState()
	gs.ActionsReducedRootSnapshots = append(
		gs.ActionsReducedRootSnapshots,
		types.ActionsReducedRootSnapshot{
			CosmosBlockHeight:  10,
			ActionsReducedRoot: canonicalRoot(42),
		},
	)

	bz, err := json.Marshal(gs)
	require.NoError(t, err)

	var roundTripped types.GenesisState
	require.NoError(t, json.Unmarshal(bz, &roundTripped))
	require.Equal(t, *gs, roundTripped)
	require.NoError(t, roundTripped.Validate())
}
