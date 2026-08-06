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
	params := types.DefaultTestParams()

	return &types.GenesisState{
		Params:                      params,
		BridgeState:                 types.NewInitialBridgeState(params.StartBlockHeight),
		ActionsReducedRootSnapshots: types.DefaultActionsReducedRootSnapshots(),
	}
}

func canonicalRoot(v uint64) []byte {
	return minafield.NewField().FromUint64(v).Bytes()
}

func canonicalActionHash(v uint64) string {
	return minafield.NewField().FromUint64(v).String()
}

func nonCanonicalRoot() []byte {
	return bytes.Repeat([]byte{0xff}, minafield.NewField().ElementSize())
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
			name: "negative valid action hashes cosmos block height",
			mutate: func(gs *types.GenesisState) {
				gs.BridgeState.ValidActionHashesCosmosBlockHeight = -1
			},
			wantErr: types.ErrInvalidBridgeStateHeight,
		},
		{
			name: "negative start mina height",
			mutate: func(gs *types.GenesisState) {
				gs.BridgeState.StartMinaHeight = -1
			},
			wantErr: types.ErrInvalidBridgeStateHeight,
		},
		{
			name: "start mina height after latest fetched mina height",
			mutate: func(gs *types.GenesisState) {
				gs.BridgeState.StartMinaHeight = gs.BridgeState.LatestFetchedMinaHeight + 1
			},
			wantErr: types.ErrInvalidBridgeStateHeight,
		},
		{
			name: "start block height must be positive",
			mutate: func(gs *types.GenesisState) {
				gs.Params.StartBlockHeight = 0
			},
			wantErr: types.ErrStartBlockHeightMustBeGreaterThanZero,
		},
		{
			name: "empty valid action hash is invalid",
			mutate: func(gs *types.GenesisState) {
				gs.BridgeState.ValidActionHashes = []string{""}
			},
			wantErr: types.ErrInvalidValidActionHash,
		},
		{
			name: "whitespace padded valid action hash is invalid",
			mutate: func(gs *types.GenesisState) {
				gs.BridgeState.ValidActionHashes = []string{" 42 "}
			},
			wantErr: types.ErrInvalidValidActionHash,
		},
		{
			name: "latest fetched before start block is invalid",
			mutate: func(gs *types.GenesisState) {
				gs.Params.StartBlockHeight = 500_000
				gs.BridgeState = types.NewInitialBridgeState(gs.Params.StartBlockHeight)
				gs.BridgeState.LatestFetchedMinaHeight--
				gs.BridgeState.StartMinaHeight = gs.BridgeState.LatestFetchedMinaHeight
			},
			wantErr: types.ErrLatestFetchedMinaHeightBeforeStartBlock,
		},
		{
			name: "start mina height before start block is invalid",
			mutate: func(gs *types.GenesisState) {
				gs.Params.StartBlockHeight = 500_000
				gs.BridgeState = types.NewInitialBridgeState(gs.Params.StartBlockHeight)
				gs.BridgeState.StartMinaHeight--
			},
			wantErr: types.ErrInvalidBridgeStateHeight,
		},
		{
			name: "empty snapshots",
			mutate: func(gs *types.GenesisState) {
				gs.ActionsReducedRootSnapshots = nil
			},
			wantErr: types.ErrEmptyActionsReducedRootSnapshots,
		},
		{
			name: "too many snapshots",
			mutate: func(gs *types.GenesisState) {
				gs.ActionsReducedRootSnapshots = nil
				for i := int64(1); i <= gs.Params.ActionsReducedRootSnapshotWindowSize+1; i++ {
					gs.ActionsReducedRootSnapshots = append(
						gs.ActionsReducedRootSnapshots,
						types.ActionsReducedRootSnapshot{
							CosmosBlockHeight:  i,
							ActionsReducedRoot: canonicalRoot(uint64(i)),
						},
					)
				}
			},
			wantErr: types.ErrTooManyActionsReducedRootSnapshots,
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

func TestGenesisStateValidateAcceptsRollingWindowWithoutHeightZero(t *testing.T) {
	gs := validGenesisState()
	gs.ActionsReducedRootSnapshots = []types.ActionsReducedRootSnapshot{
		{CosmosBlockHeight: 10, ActionsReducedRoot: canonicalRoot(10)},
		{CosmosBlockHeight: 11, ActionsReducedRoot: canonicalRoot(11)},
		{CosmosBlockHeight: 12, ActionsReducedRoot: canonicalRoot(12)},
		{CosmosBlockHeight: 13, ActionsReducedRoot: canonicalRoot(13)},
	}

	require.NoError(t, gs.Validate())
}

func TestGenesisStateValidateAcceptsCustomStartBlockHeight(t *testing.T) {
	gs := validGenesisState()
	gs.Params.StartBlockHeight = 500_000
	gs.BridgeState = types.NewInitialBridgeState(gs.Params.StartBlockHeight)

	require.NoError(t, gs.Validate())
}

func TestGenesisStateValidateAcceptsCanonicalValidActionHashes(t *testing.T) {
	gs := validGenesisState()
	gs.BridgeState.ValidActionHashes = []string{
		canonicalActionHash(42),
		canonicalActionHash(43),
	}

	require.NoError(t, gs.Validate())
}

func TestDefaultGenesisJSONRoundTrip(t *testing.T) {
	gs := validGenesisState()

	bz, err := json.Marshal(gs)
	require.NoError(t, err)

	var roundTripped types.GenesisState
	require.NoError(t, json.Unmarshal(bz, &roundTripped))
	require.Equal(t, *gs, roundTripped)
	require.NoError(t, roundTripped.Validate())
}

func TestGenesisStateJSONRoundTripWithCustomCanonicalSnapshotRoot(t *testing.T) {
	gs := validGenesisState()
	gs.BridgeState.LatestFetchedMinaHeight = 43
	gs.BridgeState.ValidActionHashes = []string{
		canonicalActionHash(42),
		canonicalActionHash(43),
	}
	gs.BridgeState.ValidActionHashesCosmosBlockHeight = 77
	gs.BridgeState.StartMinaHeight = 41
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
