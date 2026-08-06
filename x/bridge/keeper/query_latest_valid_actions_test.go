package keeper_test

import (
	"math/big"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/node101-io/pulsar-chain/x/bridge/keeper"
	"github.com/node101-io/pulsar-chain/x/bridge/types"

	minafield "github.com/node101-io/mina-signer-go/field"
	merkle "github.com/node101-io/mina-signer-go/merklelist"
)

func TestLatestValidActionHashesInvalidArgumentFail(t *testing.T) {
	f := initFixture(t, nil, nil, nil)

	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err := qs.LatestValidActionHashes(f.ctx, nil)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestLatestValidActionHashesNotFound(t *testing.T) {
	f := initFixture(t, nil, nil, nil)

	qs := keeper.NewQueryServerImpl(f.keeper)

	_, err := qs.LatestValidActionHashes(f.ctx, &types.QueryLatestValidActionHashesRequest{})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.NotFound, st.Code())
}

func TestLatestValidActionHashesSuccessWithEmptyHashes(t *testing.T) {
	f := initFixture(t, nil, nil, nil)

	require.NoError(t, f.keeper.BridgeState.Set(f.ctx, types.BridgeState{
		LatestFetchedMinaHeight: 0,
		ValidActionHashes:       nil,
	}))

	qs := keeper.NewQueryServerImpl(f.keeper)

	response, err := qs.LatestValidActionHashes(f.ctx, &types.QueryLatestValidActionHashesRequest{})
	require.NoError(t, err)
	require.Equal(t, &types.QueryLatestValidActionHashesResponse{
		LatestFetchedMinaHeight:            0,
		ValidActionHashes:                  nil,
		StartMinaHeight:                    0,
		ValidActionHashesCosmosBlockHeight: 0,
	}, response)
}

// Wrapper ordering is part of the root contract.
// The query must return hashes in the exact append order used for the Merkle list.
func TestLatestValidActionHashesPreservesMerkleAppendOrder(t *testing.T) {
	// Use one registered Mina key so both wrapper actions are accepted and
	// appended into the bridge Merkle list.
	feePayer, cosmosPubKey, _ := newUserMapping(t)

	// The two actions differ only by amount so the expected order is easy to
	// track through hashing, state storage, and the query response.
	action1 := types.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      5,
	}
	action2 := types.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      9,
	}

	// The wrapper stub returns actions in this exact order. The bridge must not
	// reorder them before storing hashes or updating the root.
	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 11,
		actions:         []types.Action{action1, action2},
	}

	bankKeeper := NewMockBankKeeper()
	keyRegistryKeeper := NewMockKeyregistryKeeper()
	keyRegistryKeeper.register(feePayer, cosmosPubKey)

	f := initFixture(t, bankKeeper, client, keyRegistryKeeper)
	seedPushNewActionsState(t, f, 10)

	// Process the batch so the keeper records both the Merkle root and the query
	// payload from the wrapper-provided action order.
	_, err := keeper.NewMsgServerImpl(f.keeper).PushNewActions(
		f.ctx,
		&types.MsgPushNewActions{
			Creator:         authorityString(t, f.addressCodec),
			MinaBlockHeight: 11,
		},
	)
	require.NoError(t, err)

	// Read the stored hashes back through the public query surface.
	queryServer := keeper.NewQueryServerImpl(f.keeper)
	response, err := queryServer.LatestValidActionHashes(f.ctx, &types.QueryLatestValidActionHashesRequest{})
	require.NoError(t, err)

	// Compute the expected field-string hashes directly from the original
	// actions so the response can be checked without depending on keeper state.
	action1Field, err := action1.ToFieldElement()
	require.NoError(t, err)
	action2Field, err := action2.ToFieldElement()
	require.NoError(t, err)

	// The query response must preserve append order and must not sort hashes.
	require.Equal(t, int64(11), response.LatestFetchedMinaHeight)
	require.Equal(t, int64(10), response.StartMinaHeight)
	require.Equal(t, int64(0), response.ValidActionHashesCosmosBlockHeight)
	require.Equal(t, []string{
		action1Field.String(),
		action2Field.String(),
	}, response.ValidActionHashes)

	gotRoot := latestActionsReducedRoot(t, f)
	rootFromQueryOrder := actionsReducedRootFromHashes(t, response.ValidActionHashes...)
	rootFromReverseOrder := actionsReducedRootFromHashes(t, response.ValidActionHashes[1], response.ValidActionHashes[0])

	// A query consumer rebuilding the list in query order must get the same root.
	// Reversing the order must produce a different root.
	require.Equal(t, expectedActionsReducedRoot(t, action1, action2), gotRoot)
	require.Equal(t, gotRoot, rootFromQueryOrder)
	require.NotEqual(t, gotRoot, rootFromReverseOrder)
}

// Successful pushes in the same Cosmos block must append into one cumulative
// query batch in transaction execution order.
func TestLatestValidActionHashesAppendsSuccessfulPushesWithinSameCosmosBlock(t *testing.T) {
	feePayer, cosmosPubKey, _ := newUserMapping(t)

	action1 := types.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      5,
	}
	action2 := types.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      9,
	}
	action3 := types.Action{
		BlockHeight: 12,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      13,
	}

	// Each PushNewActions call sees its own wrapper batch, but the query must
	// expose the cumulative same-block append order across both successful txs.
	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 12,
		actionsInRangeSource: scriptedActionsInRangeSource{
			{latestFetchedMinaHeight: 10, targetMinaHeight: 11}: {
				actions: []types.Action{action1, action2},
			},
			{latestFetchedMinaHeight: 11, targetMinaHeight: 12}: {
				actions: []types.Action{action3},
			},
		},
	}

	bankKeeper := NewMockBankKeeper()
	keyRegistryKeeper := NewMockKeyregistryKeeper()
	keyRegistryKeeper.register(feePayer, cosmosPubKey)

	f := initFixture(t, bankKeeper, client, keyRegistryKeeper)
	seedPushNewActionsState(t, f, 10)
	f.ctx = sdk.UnwrapSDKContext(f.ctx).WithBlockHeight(77)

	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err := ms.PushNewActions(f.ctx, &types.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 11,
	})
	require.NoError(t, err)

	_, err = ms.PushNewActions(f.ctx, &types.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 12,
	})
	require.NoError(t, err)

	queryServer := keeper.NewQueryServerImpl(f.keeper)
	response, err := queryServer.LatestValidActionHashes(f.ctx, &types.QueryLatestValidActionHashesRequest{})
	require.NoError(t, err)

	action1Field, err := action1.ToFieldElement()
	require.NoError(t, err)
	action2Field, err := action2.ToFieldElement()
	require.NoError(t, err)
	action3Field, err := action3.ToFieldElement()
	require.NoError(t, err)

	require.Equal(t, int64(12), response.LatestFetchedMinaHeight)
	require.Equal(t, int64(10), response.StartMinaHeight)
	require.Equal(t, int64(77), response.ValidActionHashesCosmosBlockHeight)
	require.Equal(t, []string{
		action1Field.String(),
		action2Field.String(),
		action3Field.String(),
	}, response.ValidActionHashes)

	// The same-block cumulative query payload must rebuild the final root.
	gotRoot := latestActionsReducedRoot(t, f)
	require.Equal(t, expectedActionsReducedRoot(t, action1, action2, action3), gotRoot)
	require.Equal(t, gotRoot, actionsReducedRootFromHashes(t, response.ValidActionHashes...))

	state, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(77), state.ValidActionHashesCosmosBlockHeight)
	require.Equal(t, int64(10), state.StartMinaHeight)
}

// The first successful push in the next Cosmos block must start a new visible batch.
func TestLatestValidActionHashesResetsBatchOnNextCosmosBlock(t *testing.T) {
	feePayer, cosmosPubKey, _ := newUserMapping(t)

	action1 := types.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      5,
	}
	action2 := types.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      9,
	}
	action3 := types.Action{
		BlockHeight: 12,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      13,
	}
	action4 := types.Action{
		BlockHeight: 13,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      21,
	}

	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 13,
		actionsInRangeSource: scriptedActionsInRangeSource{
			{latestFetchedMinaHeight: 10, targetMinaHeight: 11}: {
				actions: []types.Action{action1, action2},
			},
			{latestFetchedMinaHeight: 11, targetMinaHeight: 12}: {
				actions: []types.Action{action3},
			},
			{latestFetchedMinaHeight: 12, targetMinaHeight: 13}: {
				actions: []types.Action{action4},
			},
		},
	}

	bankKeeper := NewMockBankKeeper()
	keyRegistryKeeper := NewMockKeyregistryKeeper()
	keyRegistryKeeper.register(feePayer, cosmosPubKey)

	f := initFixture(t, bankKeeper, client, keyRegistryKeeper)
	seedPushNewActionsState(t, f, 10)

	ms := keeper.NewMsgServerImpl(f.keeper)

	// Build one cumulative batch in Cosmos block 77 first.
	f.ctx = sdk.UnwrapSDKContext(f.ctx).WithBlockHeight(77)
	_, err := ms.PushNewActions(f.ctx, &types.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 11,
	})
	require.NoError(t, err)
	_, err = ms.PushNewActions(f.ctx, &types.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 12,
	})
	require.NoError(t, err)

	rootAfterBlock77 := latestActionsReducedRoot(t, f)

	// The next Cosmos block must expose only the new block's batch.
	f.ctx = sdk.UnwrapSDKContext(f.ctx).WithBlockHeight(78)
	_, err = ms.PushNewActions(f.ctx, &types.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 13,
	})
	require.NoError(t, err)

	queryServer := keeper.NewQueryServerImpl(f.keeper)
	response, err := queryServer.LatestValidActionHashes(f.ctx, &types.QueryLatestValidActionHashesRequest{})
	require.NoError(t, err)

	action4Field, err := action4.ToFieldElement()
	require.NoError(t, err)

	require.Equal(t, int64(13), response.LatestFetchedMinaHeight)
	require.Equal(t, int64(12), response.StartMinaHeight)
	require.Equal(t, int64(78), response.ValidActionHashesCosmosBlockHeight)
	require.Equal(t, []string{action4Field.String()}, response.ValidActionHashes)

	// The query resets to the new block's batch, while the chain root still
	// includes all historical actions committed before this block.
	gotRoot := latestActionsReducedRoot(t, f)
	require.Equal(t, expectedActionsReducedRoot(t, action1, action2, action3, action4), gotRoot)
	require.Equal(t, gotRoot, actionsReducedRootFromBaseAndHashes(t, rootAfterBlock77, response.ValidActionHashes...))

	state, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(78), state.ValidActionHashesCosmosBlockHeight)
	require.Equal(t, int64(12), state.StartMinaHeight)
}

// If no PushNewActions runs in the next Cosmos block, the query still returns
// the previous batch and must identify the original source block correctly.
func TestLatestValidActionHashesKeepsPreviousBatchSourceHeightAcrossLaterEmptyBlock(t *testing.T) {
	feePayer, cosmosPubKey, _ := newUserMapping(t)

	action1 := types.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      5,
	}
	action2 := types.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      9,
	}
	action3 := types.Action{
		BlockHeight: 12,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      13,
	}

	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 12,
		actionsInRangeSource: scriptedActionsInRangeSource{
			{latestFetchedMinaHeight: 10, targetMinaHeight: 11}: {
				actions: []types.Action{action1, action2},
			},
			{latestFetchedMinaHeight: 11, targetMinaHeight: 12}: {
				actions: []types.Action{action3},
			},
		},
	}

	bankKeeper := NewMockBankKeeper()
	keyRegistryKeeper := NewMockKeyregistryKeeper()
	keyRegistryKeeper.register(feePayer, cosmosPubKey)

	f := initFixture(t, bankKeeper, client, keyRegistryKeeper)
	seedPushNewActionsState(t, f, 10)

	ms := keeper.NewMsgServerImpl(f.keeper)

	// Build the cumulative batch in Cosmos block 77.
	f.ctx = sdk.UnwrapSDKContext(f.ctx).WithBlockHeight(77)
	_, err := ms.PushNewActions(f.ctx, &types.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 11,
	})
	require.NoError(t, err)
	_, err = ms.PushNewActions(f.ctx, &types.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 12,
	})
	require.NoError(t, err)

	// Query one Cosmos block later without any new push. The batch should still
	// identify block 77 as its source.
	f.ctx = sdk.UnwrapSDKContext(f.ctx).WithBlockHeight(78)
	queryServer := keeper.NewQueryServerImpl(f.keeper)
	response, err := queryServer.LatestValidActionHashes(f.ctx, &types.QueryLatestValidActionHashesRequest{})
	require.NoError(t, err)

	action1Field, err := action1.ToFieldElement()
	require.NoError(t, err)
	action2Field, err := action2.ToFieldElement()
	require.NoError(t, err)
	action3Field, err := action3.ToFieldElement()
	require.NoError(t, err)

	require.Equal(t, int64(12), response.LatestFetchedMinaHeight)
	require.Equal(t, int64(10), response.StartMinaHeight)
	require.Equal(t, int64(77), response.ValidActionHashesCosmosBlockHeight)
	require.Equal(t, []string{
		action1Field.String(),
		action2Field.String(),
		action3Field.String(),
	}, response.ValidActionHashes)

	rootBeforeBatch, err := f.keeper.GetActionsReducedRootAtHeight(f.ctx, response.ValidActionHashesCosmosBlockHeight-1)
	require.NoError(t, err)

	gotRoot := latestActionsReducedRoot(t, f)
	require.Equal(t, expectedActionsReducedRoot(t, action1, action2, action3), gotRoot)
	require.Equal(t, gotRoot, actionsReducedRootFromBaseAndHashes(t, rootBeforeBatch, response.ValidActionHashes...))

	state, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(77), state.ValidActionHashesCosmosBlockHeight)
	require.Equal(t, int64(10), state.StartMinaHeight)
}

// A failed later tx in the same Cosmos block must not corrupt the last successful batch.
func TestLatestValidActionHashesFailedSecondPushKeepsPreviousSuccessfulBatch(t *testing.T) {
	feePayer, cosmosPubKey, _ := newUserMapping(t)

	successAction := types.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      5,
	}
	failingAction := types.Action{
		BlockHeight: 12,
		FeePayer:    feePayer,
		ActionType:  types.ActionType_ACTION_TYPE_WITHDRAW,
		Amount:      7,
	}

	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 12,
		actionsInRangeSource: scriptedActionsInRangeSource{
			{latestFetchedMinaHeight: 10, targetMinaHeight: 11}: {
				actions: []types.Action{successAction},
			},
			{latestFetchedMinaHeight: 11, targetMinaHeight: 12}: {
				actions: []types.Action{failingAction},
			},
		},
	}

	bankKeeper := NewMockBankKeeper()
	bankKeeper.spendable = sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 50))
	bankKeeper.sendToModuleErr = status.Error(codes.Internal, "send to module failed")

	keyRegistryKeeper := NewMockKeyregistryKeeper()
	keyRegistryKeeper.register(feePayer, cosmosPubKey)

	f := initFixture(t, bankKeeper, client, keyRegistryKeeper)
	seedPushNewActionsState(t, f, 10)
	f.ctx = sdk.UnwrapSDKContext(f.ctx).WithBlockHeight(77)

	ms := keeper.NewMsgServerImpl(f.keeper)

	_, err := ms.PushNewActions(f.ctx, &types.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 11,
	})
	require.NoError(t, err)

	rootAfterFirstSuccess := latestActionsReducedRoot(t, f)

	_, err = ms.PushNewActions(f.ctx, &types.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 12,
	})
	require.Error(t, err)

	queryServer := keeper.NewQueryServerImpl(f.keeper)
	response, err := queryServer.LatestValidActionHashes(f.ctx, &types.QueryLatestValidActionHashesRequest{})
	require.NoError(t, err)

	successField, err := successAction.ToFieldElement()
	require.NoError(t, err)

	require.Equal(t, int64(11), response.LatestFetchedMinaHeight)
	require.Equal(t, int64(10), response.StartMinaHeight)
	require.Equal(t, int64(77), response.ValidActionHashesCosmosBlockHeight)
	require.Equal(t, []string{successField.String()}, response.ValidActionHashes)
	require.Equal(t, rootAfterFirstSuccess, latestActionsReducedRoot(t, f))

	state, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(77), state.ValidActionHashesCosmosBlockHeight)
	require.Equal(t, int64(10), state.StartMinaHeight)
}

// actionsReducedRootFromHashes rebuilds the Merkle root exactly the way an RPC
// consumer would: decode decimal field strings, append in order, and take Root().
func actionsReducedRootFromHashes(t *testing.T, hashes ...string) []byte {
	t.Helper()

	return actionsReducedRootFromBaseAndHashes(t, types.DefaultActionsReducedRoot(), hashes...)
}

// actionsReducedRootFromBaseAndHashes rebuilds the next Merkle root from a
// previously committed root plus the ordered query batch that should be appended to it.
func actionsReducedRootFromBaseAndHashes(t *testing.T, baseRoot []byte, hashes ...string) []byte {
	t.Helper()

	list, err := merkle.NewMerkleListFromRoot(types.ActionsReducedRootMerkleListPrefixV1, baseRoot)
	require.NoError(t, err)
	fieldCodec := minafield.NewField()

	for _, hash := range hashes {
		// Query responses expose field elements as decimal strings.
		n, ok := new(big.Int).SetString(hash, 10)
		require.True(t, ok)
		require.LessOrEqual(t, len(n.Bytes()), fieldCodec.ElementSize())

		fixed := make([]byte, fieldCodec.ElementSize())
		copy(fixed[len(fixed)-len(n.Bytes()):], n.Bytes())

		fieldElement, err := fieldCodec.FromBytes(fixed)
		require.NoError(t, err)
		require.NoError(t, list.Append(fieldElement.Bytes()))
	}

	return list.Root()
}
