package keeper_test

import (
	"math/big"
	"testing"

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
		LatestFetchedMinaHeight: 0,
		ValidActionHashes:       nil,
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

// actionsReducedRootFromHashes rebuilds the Merkle root exactly the way an RPC
// consumer would: decode decimal field strings, append in order, and take Root().
func actionsReducedRootFromHashes(t *testing.T, hashes ...string) []byte {
	t.Helper()

	list := merkle.NewMerkleList(types.ActionsReducedRootMerkleListPrefixV1)
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
