package types_test

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/bronlabs/bron-crypto/pkg/base/curves/pasta"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
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

func pallasBaseFieldModulus() *big.Int {
	return new(big.Int).Set(pasta.NewPallasCurve().ToElliptic().Params().P)
}

func oversizedDecimal() string {
	fieldBytes := minafield.NewField().ElementSize()
	return new(big.Int).Lsh(big.NewInt(1), uint(fieldBytes*8)).String()
}

func newProtoCodec() *codec.ProtoCodec {
	return codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
}

func setRuntimeBatch(
	gs *types.GenesisState,
	sourceCosmosHeight int64,
	startMinaHeight int64,
	latestFetchedMinaHeight int64,
	hashes []string,
) {
	gs.BridgeState = types.BridgeState{
		LatestFetchedMinaHeight:       latestFetchedMinaHeight,
		ActionHashes:                  hashes,
		ActionHashesCosmosBlockHeight: sourceCosmosHeight,
		StartMinaHeight:               startMinaHeight,
	}
	gs.ActionsReducedRootSnapshots = []types.ActionsReducedRootSnapshot{
		{
			CosmosBlockHeight:  sourceCosmosHeight,
			ActionsReducedRoot: canonicalRoot(uint64(sourceCosmosHeight)),
		},
	}
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
			name: "negative action hashes cosmos block height",
			mutate: func(gs *types.GenesisState) {
				gs.BridgeState.ActionHashesCosmosBlockHeight = -1
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
			name: "valid runtime batch with non empty hashes",
			mutate: func(gs *types.GenesisState) {
				setRuntimeBatch(gs, 77, 41, 43, []string{
					canonicalActionHash(42),
					canonicalActionHash(43),
				})
			},
		},
		{
			name: "valid runtime batch with empty hashes",
			mutate: func(gs *types.GenesisState) {
				setRuntimeBatch(gs, 77, 41, 43, nil)
			},
		},
		{
			name: "initial batch rejects non empty hashes",
			mutate: func(gs *types.GenesisState) {
				gs.BridgeState.ActionHashes = []string{canonicalActionHash(42)}
			},
			wantErr: types.ErrInvalidActionBatch,
		},
		{
			name: "initial batch rejects advanced mina range",
			mutate: func(gs *types.GenesisState) {
				gs.BridgeState.LatestFetchedMinaHeight = 43
				gs.BridgeState.StartMinaHeight = 41
			},
			wantErr: types.ErrInvalidActionBatch,
		},
		{
			name: "runtime batch must advance mina cursor",
			mutate: func(gs *types.GenesisState) {
				setRuntimeBatch(gs, 77, 43, 43, nil)
			},
			wantErr: types.ErrInvalidActionBatch,
		},
		{
			name: "empty action hash is invalid",
			mutate: func(gs *types.GenesisState) {
				setRuntimeBatch(gs, 77, 41, 43, nil)
				gs.BridgeState.ActionHashes = []string{""}
			},
			wantErr: types.ErrInvalidActionHash,
		},
		{
			name: "whitespace padded action hash is invalid",
			mutate: func(gs *types.GenesisState) {
				setRuntimeBatch(gs, 77, 41, 43, nil)
				gs.BridgeState.ActionHashes = []string{" 42 "}
			},
			wantErr: types.ErrInvalidActionHash,
		},
		{
			name: "non decimal action hash is invalid",
			mutate: func(gs *types.GenesisState) {
				setRuntimeBatch(gs, 77, 41, 43, nil)
				gs.BridgeState.ActionHashes = []string{"abc"}
			},
			wantErr: types.ErrInvalidActionHash,
		},
		{
			name: "negative action hash is invalid",
			mutate: func(gs *types.GenesisState) {
				setRuntimeBatch(gs, 77, 41, 43, nil)
				gs.BridgeState.ActionHashes = []string{"-1"}
			},
			wantErr: types.ErrInvalidActionHash,
		},
		{
			name: "leading zero action hash is invalid",
			mutate: func(gs *types.GenesisState) {
				setRuntimeBatch(gs, 77, 41, 43, nil)
				gs.BridgeState.ActionHashes = []string{"01"}
			},
			wantErr: types.ErrInvalidActionHash,
		},
		{
			name: "field modulus action hash is invalid",
			mutate: func(gs *types.GenesisState) {
				setRuntimeBatch(gs, 77, 41, 43, nil)
				gs.BridgeState.ActionHashes = []string{pallasBaseFieldModulus().String()}
			},
			wantErr: types.ErrInvalidActionHash,
		},
		{
			name: "above field modulus action hash is invalid",
			mutate: func(gs *types.GenesisState) {
				setRuntimeBatch(gs, 77, 41, 43, nil)
				above := new(big.Int).Add(pallasBaseFieldModulus(), big.NewInt(1))
				gs.BridgeState.ActionHashes = []string{above.String()}
			},
			wantErr: types.ErrInvalidActionHash,
		},
		{
			name: "oversized decimal action hash is invalid",
			mutate: func(gs *types.GenesisState) {
				setRuntimeBatch(gs, 77, 41, 43, nil)
				gs.BridgeState.ActionHashes = []string{oversizedDecimal()}
			},
			wantErr: types.ErrInvalidActionHash,
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
				setRuntimeBatch(gs, 500_000, 499_998, 500_000, nil)
			},
			wantErr: types.ErrInvalidBridgeStateHeight,
		},
		{
			name: "batch cosmos height must match newest snapshot height",
			mutate: func(gs *types.GenesisState) {
				setRuntimeBatch(gs, 77, 41, 43, []string{canonicalActionHash(42)})
				gs.ActionsReducedRootSnapshots = []types.ActionsReducedRootSnapshot{
					{
						CosmosBlockHeight:  78,
						ActionsReducedRoot: canonicalRoot(78),
					},
				}
			},
			wantErr: types.ErrInvalidActionBatch,
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
	setRuntimeBatch(gs, 10, 41, 43, nil)
	gs.ActionsReducedRootSnapshots[0].ActionsReducedRoot = canonicalRoot(42)

	require.NoError(t, gs.Validate())
}

func TestGenesisStateValidateAcceptsRollingWindowWithoutHeightZero(t *testing.T) {
	gs := validGenesisState()
	gs.BridgeState = types.BridgeState{
		LatestFetchedMinaHeight:       43,
		ActionHashesCosmosBlockHeight: 13,
		StartMinaHeight:               41,
	}
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

func TestGenesisStateValidateAcceptsCanonicalActionHashes(t *testing.T) {
	gs := validGenesisState()
	setRuntimeBatch(gs, 77, 41, 43, []string{
		canonicalActionHash(0),
		canonicalActionHash(42),
		canonicalActionHash(43),
	})

	require.NoError(t, gs.Validate())
}

func TestDefaultGenesisJSONRoundTrip(t *testing.T) {
	gs := validGenesisState()
	cdc := newProtoCodec()

	bz := cdc.MustMarshalJSON(gs)

	var roundTripped types.GenesisState
	require.NoError(t, cdc.UnmarshalJSON(bz, &roundTripped))
	require.Equal(t, *gs, roundTripped)
	require.NoError(t, roundTripped.Validate())

	var payload map[string]any
	require.NoError(t, json.Unmarshal(bz, &payload))

	bridgeState, ok := payload["bridge_state"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "0", bridgeState["latest_fetched_mina_height"])
	require.Equal(t, "0", bridgeState["action_hashes_cosmos_block_height"])
	require.Equal(t, "0", bridgeState["start_mina_height"])

	hashes, ok := bridgeState["action_hashes"].([]any)
	require.True(t, ok)
	require.Len(t, hashes, 0)
}

func TestGenesisStateJSONRoundTripWithCustomCanonicalSnapshotRoot(t *testing.T) {
	gs := validGenesisState()
	gs.BridgeState.LatestFetchedMinaHeight = 43
	gs.BridgeState.ActionHashes = []string{
		canonicalActionHash(42),
		canonicalActionHash(43),
	}
	gs.BridgeState.ActionHashesCosmosBlockHeight = 77
	gs.BridgeState.StartMinaHeight = 41
	gs.ActionsReducedRootSnapshots = append(
		gs.ActionsReducedRootSnapshots,
		types.ActionsReducedRootSnapshot{
			CosmosBlockHeight:  77,
			ActionsReducedRoot: canonicalRoot(42),
		},
	)
	cdc := newProtoCodec()

	bz := cdc.MustMarshalJSON(gs)

	var roundTripped types.GenesisState
	require.NoError(t, cdc.UnmarshalJSON(bz, &roundTripped))
	require.Equal(t, *gs, roundTripped)
	require.NoError(t, roundTripped.Validate())

	var payload map[string]any
	require.NoError(t, json.Unmarshal(bz, &payload))

	bridgeState, ok := payload["bridge_state"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "43", bridgeState["latest_fetched_mina_height"])
	require.Equal(t, "77", bridgeState["action_hashes_cosmos_block_height"])
	require.Equal(t, "41", bridgeState["start_mina_height"])

	hashes, ok := bridgeState["action_hashes"].([]any)
	require.True(t, ok)
	require.Equal(t, []any{canonicalActionHash(42), canonicalActionHash(43)}, hashes)
}
