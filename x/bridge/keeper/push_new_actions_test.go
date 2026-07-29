package keeper_test

import (
	"context"
	"testing"

	"cosmossdk.io/core/address"
	storetypes "cosmossdk.io/store/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	bridgekeeper "github.com/node101-io/pulsar-chain/x/bridge/keeper"
	module "github.com/node101-io/pulsar-chain/x/bridge/module"
	bridgetypes "github.com/node101-io/pulsar-chain/x/bridge/types"
)

type stubArchiveWrapperQueryClient struct {
	getMinaBlockHeightCalls int
	minaBlockHeight         int64
	minaBlockHeightErr      error

	actions    []bridgetypes.Action
	actionsErr error

	getActionsCalls  int
	gotLatestFetched int64
	gotTarget        int64
}

func (c *stubArchiveWrapperQueryClient) GetMinaBlockHeight(context.Context) (int64, error) {
	c.getMinaBlockHeightCalls++
	return c.minaBlockHeight, c.minaBlockHeightErr
}

func latestActionsReducedRoot(t *testing.T, f *fixture) []byte {
	t.Helper()

	root, err := f.keeper.GetLatestActionsReducedRoot(f.ctx)
	require.NoError(t, err)

	return append([]byte(nil), root...)
}

func (c *stubArchiveWrapperQueryClient) GetActionsInRange(
	_ context.Context,
	latestFetchedMinaHeight int64,
	targetMinaHeight int64,
) ([]bridgetypes.Action, error) {
	c.getActionsCalls++
	c.gotLatestFetched = latestFetchedMinaHeight
	c.gotTarget = targetMinaHeight
	return c.actions, c.actionsErr
}

func initFixtureWithArchiveWrapperClient(t *testing.T, client bridgekeeper.ArchiveWrapperQueryClient) *fixture {
	t.Helper()

	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(bridgetypes.StoreKey)

	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx

	authority := authtypes.NewModuleAddress(bridgetypes.GovModuleName)

	k := bridgekeeper.NewKeeper(
		storeService,
		encCfg.Codec,
		addressCodec,
		authority,
		nil,
		nil,
		client,
	)

	require.NoError(t, k.Params.Set(ctx, validBridgeParams()))

	return &fixture{
		ctx:          ctx,
		keeper:       k,
		addressCodec: addressCodec,
	}
}

func seedPushNewActionsState(t *testing.T, f *fixture, latestFetchedMinaHeight int64) {
	t.Helper()

	require.NoError(t, f.keeper.BridgeState.Set(f.ctx, bridgetypes.BridgeState{
		LatestFetchedMinaHeight: latestFetchedMinaHeight,
	}))
	require.NoError(t, f.keeper.ActionsReducedRootSnapshots.Set(
		f.ctx,
		0,
		bridgetypes.DefaultActionsReducedRoot(),
	))
}

func authorityString(t *testing.T, codec address.Codec) string {
	t.Helper()

	authority := authtypes.NewModuleAddress(bridgetypes.GovModuleName)
	authorityStr, err := codec.BytesToString(authority)
	require.NoError(t, err)

	return authorityStr
}

func TestPushNewActionsAcceptsTargetAtIndexedCursor(t *testing.T) {
	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 10,
	}

	f := initFixtureWithArchiveWrapperClient(t, client)
	seedPushNewActionsState(t, f, 9)

	ms := bridgekeeper.NewMsgServerImpl(f.keeper)

	resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 10,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	state, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(10), state.LatestFetchedMinaHeight)

	require.Equal(t, 1, client.getActionsCalls)
	require.Equal(t, int64(9), client.gotLatestFetched)
	require.Equal(t, int64(10), client.gotTarget)
}

func TestPushNewActionsRejectsTargetAboveIndexedCursor(t *testing.T) {
	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 10,
	}

	f := initFixtureWithArchiveWrapperClient(t, client)
	seedPushNewActionsState(t, f, 9)

	ms := bridgekeeper.NewMsgServerImpl(f.keeper)

	resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 11,
	})

	require.ErrorIs(t, err, bridgetypes.ErrMinaBlockNotFinalized)
	require.Nil(t, resp)
	require.Equal(t, 0, client.getActionsCalls)

	state, getErr := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, getErr)
	require.Equal(t, int64(9), state.LatestFetchedMinaHeight)
}

func TestPushNewActionsBootstrapStartsFromConfiguredStartBlockHeight(t *testing.T) {
	const startBlockHeight int64 = 500_000
	const targetMinaHeight int64 = 500_100

	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: targetMinaHeight,
	}

	f := initFixtureWithArchiveWrapperClient(t, client)

	genesisState := bridgetypes.GenesisState{
		Params: bridgetypes.NewParams(
			testConfirmationDepth,
			testContractAddress,
			startBlockHeight,
		),
		BridgeState:                 bridgetypes.NewInitialBridgeState(startBlockHeight),
		ActionsReducedRootSnapshots: bridgetypes.DefaultActionsReducedRootSnapshots(),
	}

	require.NoError(t, f.keeper.InitGenesis(f.ctx, genesisState))

	ms := bridgekeeper.NewMsgServerImpl(f.keeper)

	resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: targetMinaHeight,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Equal(t, 1, client.getActionsCalls)
	require.Equal(t, startBlockHeight-1, client.gotLatestFetched)
	require.Equal(t, startBlockHeight, client.gotLatestFetched+1)
	require.Equal(t, targetMinaHeight, client.gotTarget)

	state, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, targetMinaHeight, state.LatestFetchedMinaHeight)
}
func TestPushNewActionsRejectsInvalidOrNonAdvancingTargetsWithoutMutatingState(t *testing.T) {
	testCases := []struct {
		name                    string
		latestFetchedMinaHeight int64
		targetMinaHeight        int64
		wantErr                 error
	}{
		{
			name:                    "target below cursor",
			latestFetchedMinaHeight: 10,
			targetMinaHeight:        9,
			wantErr:                 bridgetypes.ErrMinaBlockHeightMustAdvance,
		},
		{
			name:                    "target equal to cursor",
			latestFetchedMinaHeight: 10,
			targetMinaHeight:        10,
			wantErr:                 bridgetypes.ErrMinaBlockHeightMustAdvance,
		},
		{
			name:                    "target zero",
			latestFetchedMinaHeight: 10,
			targetMinaHeight:        0,
			wantErr:                 bridgetypes.ErrInvalidMinaBlockHeight,
		},
		{
			name:                    "target negative",
			latestFetchedMinaHeight: 10,
			targetMinaHeight:        -1,
			wantErr:                 bridgetypes.ErrInvalidMinaBlockHeight,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &stubArchiveWrapperQueryClient{
				minaBlockHeight: 200,
			}

			bankKeeper := NewMockBankKeeper()
			f := initFixture(t, bankKeeper, client)
			seedPushNewActionsState(t, f, tc.latestFetchedMinaHeight)

			beforeBalance := append(sdk.Coins(nil), bankKeeper.spendable...)

			beforeState, err := f.keeper.GetBridgeState(f.ctx)
			require.NoError(t, err)

			beforeRoot := latestActionsReducedRoot(t, f)

			ms := bridgekeeper.NewMsgServerImpl(f.keeper)

			resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
				Creator:         authorityString(t, f.addressCodec),
				MinaBlockHeight: tc.targetMinaHeight,
			})

			require.ErrorIs(t, err, tc.wantErr)
			require.Nil(t, resp)

			require.Equal(t, 0, client.getMinaBlockHeightCalls)
			require.Equal(t, 0, client.getActionsCalls)

			afterState, err := f.keeper.GetBridgeState(f.ctx)
			require.NoError(t, err)
			require.Equal(t, beforeState, afterState)

			afterRoot := latestActionsReducedRoot(t, f)
			require.Equal(t, beforeRoot, afterRoot)

			require.Equal(t, beforeBalance, bankKeeper.spendable)
			require.Zero(t, bankKeeper.spendableCalls)
			require.Zero(t, bankKeeper.sendCoinsFromModuleCalls)
			require.Zero(t, bankKeeper.sendCoinsToModuleCalls)
			require.Zero(t, bankKeeper.mintCoinsCalls)
			require.Zero(t, bankKeeper.burnCoinsCalls)

			require.Zero(t, client.getMinaBlockHeightCalls)
			require.Zero(t, client.getActionsCalls)
		})
	}
}

func TestPushNewActionsRejectsReplayOfAlreadyProcessedTarget(t *testing.T) {
	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 10,
	}

	f := initFixtureWithArchiveWrapperClient(t, client)
	seedPushNewActionsState(t, f, 9)

	ms := bridgekeeper.NewMsgServerImpl(f.keeper)

	firstResp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 10,
	})
	require.NoError(t, err)
	require.NotNil(t, firstResp)

	stateAfterFirst, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	rootAfterFirst := latestActionsReducedRoot(t, f)

	secondResp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 10,
	})
	require.ErrorIs(t, err, bridgetypes.ErrMinaBlockHeightMustAdvance)
	require.Nil(t, secondResp)

	stateAfterSecond, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, stateAfterFirst, stateAfterSecond)

	rootAfterSecond := latestActionsReducedRoot(t, f)
	require.Equal(t, rootAfterFirst, rootAfterSecond)

	require.Equal(t, 1, client.getMinaBlockHeightCalls)
	require.Equal(t, 1, client.getActionsCalls)
}
