package keeper_test

import (
	"context"
	"errors"
	"testing"

	"cosmossdk.io/core/address"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cometbft/cometbft/crypto/secp256k1"
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

	minaaddress "github.com/node101-io/mina-signer-go/address"
	merkle "github.com/node101-io/mina-signer-go/merklelist"
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

type mockKeyregistryKeeper struct {
	existsByMinaKey map[string]bool
	cosmosByMinaKey map[string][]byte
	hasErr          error
	getErr          error
}

func NewMockKeyregistryKeeper() *mockKeyregistryKeeper {
	return &mockKeyregistryKeeper{
		existsByMinaKey: make(map[string]bool),
		cosmosByMinaKey: make(map[string][]byte),
	}
}

func (m *mockKeyregistryKeeper) register(minaKey []byte, cosmosPubKey []byte) {
	if m.existsByMinaKey == nil {
		m.existsByMinaKey = make(map[string]bool)
	}
	if m.cosmosByMinaKey == nil {
		m.cosmosByMinaKey = make(map[string][]byte)
	}

	key := string(minaKey)
	m.existsByMinaKey[key] = true

	bz := make([]byte, len(cosmosPubKey))
	copy(bz, cosmosPubKey)
	m.cosmosByMinaKey[key] = bz
}

func (m *mockKeyregistryKeeper) UserMinaToCosmosHas(_ context.Context, minaKey []byte) (bool, error) {
	if m.hasErr != nil {
		return false, m.hasErr
	}
	return m.existsByMinaKey[string(minaKey)], nil
}

func (m *mockKeyregistryKeeper) UserGetMinaToCosmos(_ context.Context, minaKey []byte) ([]byte, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}

	bz := m.cosmosByMinaKey[string(minaKey)]
	if bz == nil {
		return nil, nil
	}

	out := make([]byte, len(bz))
	copy(out, bz)
	return out, nil
}

func mustMinaFeePayerBytes(t *testing.T) []byte {
	t.Helper()

	bz, err := minaaddress.NewAddress(testContractAddress).Marshal()
	require.NoError(t, err)

	return bz
}

func newUserMapping(t *testing.T) ([]byte, []byte, sdk.AccAddress) {
	t.Helper()

	feePayer := mustMinaFeePayerBytes(t)
	privKey := secp256k1.GenPrivKey()
	cosmosPubKey := privKey.PubKey().Bytes()
	cosmosAddr := sdk.AccAddress(privKey.PubKey().Address())

	return feePayer, cosmosPubKey, cosmosAddr
}

func expectedActionsReducedRoot(t *testing.T, actions ...bridgetypes.Action) []byte {
	t.Helper()

	list := merkle.NewMerkleList(bridgetypes.ActionsReducedRootMerkleListPrefixV1)

	for _, act := range actions {
		fieldElement, err := act.ToFieldElement()
		require.NoError(t, err)
		require.NoError(t, list.Append(fieldElement.Bytes()))
	}

	return list.Root()
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
			testMaxBlockRange,
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
			f := initFixture(t, bankKeeper, client, nil)
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
func TestPushNewActionsRejectsTargetBeyondMaxBlockRange(t *testing.T) {
	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 1_000_000,
	}

	f := initFixture(t, nil, client, nil)
	require.NoError(t, f.keeper.Params.Set(
		f.ctx,
		bridgetypes.NewParams(
			testConfirmationDepth,
			testContractAddress,
			testStartBlockHeight,
			10,
		),
	))
	seedPushNewActionsState(t, f, 100)

	beforeState, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)

	beforeRoot := latestActionsReducedRoot(t, f)

	ms := bridgekeeper.NewMsgServerImpl(f.keeper)

	resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 111,
	})

	require.ErrorIs(t, err, bridgetypes.ErrMinaBlockRangeTooLarge)
	require.Nil(t, resp)
	require.Zero(t, client.getMinaBlockHeightCalls)
	require.Zero(t, client.getActionsCalls)

	afterState, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, beforeState, afterState)

	afterRoot := latestActionsReducedRoot(t, f)
	require.Equal(t, beforeRoot, afterRoot)
}
func TestPushNewActionsAcceptsTargetAtMaxBlockRange(t *testing.T) {
	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 110,
	}

	f := initFixture(t, nil, client, nil)
	require.NoError(t, f.keeper.Params.Set(
		f.ctx,
		bridgetypes.NewParams(
			testConfirmationDepth,
			testContractAddress,
			testStartBlockHeight,
			10,
		),
	))
	seedPushNewActionsState(t, f, 100)

	ms := bridgekeeper.NewMsgServerImpl(f.keeper)

	resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 110,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 1, client.getMinaBlockHeightCalls)
	require.Equal(t, 1, client.getActionsCalls)
	require.Equal(t, int64(100), client.gotLatestFetched)
	require.Equal(t, int64(110), client.gotTarget)
}
func TestPushNewActionsSkipsNonPositiveAmountsWithoutMutatingBalanceOrRoot(t *testing.T) {
	testCases := []struct {
		name       string
		actionType bridgetypes.ActionType
		amount     int64
	}{
		{
			name:       "deposit zero amount",
			actionType: bridgetypes.ActionType_ACTION_TYPE_DEPOSIT,
			amount:     0,
		},
		{
			name:       "deposit negative amount",
			actionType: bridgetypes.ActionType_ACTION_TYPE_DEPOSIT,
			amount:     -1,
		},
		{
			name:       "withdraw zero amount",
			actionType: bridgetypes.ActionType_ACTION_TYPE_WITHDRAW,
			amount:     0,
		},
		{
			name:       "withdraw negative amount",
			actionType: bridgetypes.ActionType_ACTION_TYPE_WITHDRAW,
			amount:     -1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &stubArchiveWrapperQueryClient{
				minaBlockHeight: 11,
				actions: []bridgetypes.Action{
					{
						BlockHeight: 11,
						FeePayer:    []byte("ignored"),
						ActionType:  tc.actionType,
						Amount:      tc.amount,
					},
				},
			}

			bankKeeper := NewMockBankKeeper()
			keyRegistryKeeper := &mockKeyregistryKeeper{}

			f := initFixture(t, bankKeeper, client, keyRegistryKeeper)
			seedPushNewActionsState(t, f, 10)

			beforeBalance := append(sdk.Coins(nil), bankKeeper.spendable...)

			beforeState, err := f.keeper.GetBridgeState(f.ctx)
			require.NoError(t, err)

			beforeRoot := latestActionsReducedRoot(t, f)

			ms := bridgekeeper.NewMsgServerImpl(f.keeper)

			resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
				Creator:         authorityString(t, f.addressCodec),
				MinaBlockHeight: 11,
			})

			require.NoError(t, err)
			require.NotNil(t, resp)

			afterState, err := f.keeper.GetBridgeState(f.ctx)
			require.NoError(t, err)
			require.Equal(t, int64(11), afterState.LatestFetchedMinaHeight)
			require.NotEqual(t, beforeState.LatestFetchedMinaHeight, afterState.LatestFetchedMinaHeight)

			afterRoot := latestActionsReducedRoot(t, f)
			require.Equal(t, beforeRoot, afterRoot)

			require.Equal(t, beforeBalance, bankKeeper.spendable)
			require.Zero(t, bankKeeper.spendableCalls)
			require.Zero(t, bankKeeper.sendCoinsFromModuleCalls)
			require.Zero(t, bankKeeper.sendCoinsToModuleCalls)
			require.Zero(t, bankKeeper.mintCoinsCalls)
			require.Zero(t, bankKeeper.burnCoinsCalls)

			require.Equal(t, 1, client.getMinaBlockHeightCalls)
			require.Equal(t, 1, client.getActionsCalls)
		})
	}
}
func TestPushNewActionsUserDepositHappyPath(t *testing.T) {
	feePayer, cosmosPubKey, cosmosAddr := newUserMapping(t)

	action := bridgetypes.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  bridgetypes.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      7,
	}

	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 11,
		actions:         []bridgetypes.Action{action},
	}

	bankKeeper := NewMockBankKeeper()
	keyRegistryKeeper := NewMockKeyregistryKeeper()
	keyRegistryKeeper.register(feePayer, cosmosPubKey)

	f := initFixture(t, bankKeeper, client, keyRegistryKeeper)
	seedPushNewActionsState(t, f, 10)

	beforeRoot := latestActionsReducedRoot(t, f)

	ms := bridgekeeper.NewMsgServerImpl(f.keeper)
	resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 11,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	state, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(11), state.LatestFetchedMinaHeight)

	afterRoot := latestActionsReducedRoot(t, f)
	expectedCoins := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 7))

	require.NotEqual(t, beforeRoot, afterRoot)
	require.Equal(t, expectedActionsReducedRoot(t, action), afterRoot)

	require.Equal(t, 1, bankKeeper.mintCoinsCalls)
	require.Equal(t, bridgetypes.ModuleName, bankKeeper.lastMintModule)
	require.True(t, bankKeeper.lastMintCoins.Equal(expectedCoins))

	require.Equal(t, 1, bankKeeper.sendCoinsFromModuleCalls)
	require.Equal(t, bridgetypes.ModuleName, bankKeeper.lastSendFromModule)
	require.Equal(t, cosmosAddr, bankKeeper.lastSendToAddr)
	require.True(t, bankKeeper.lastSendFromModuleCoins.Equal(expectedCoins))

	require.Zero(t, bankKeeper.spendableCalls)
	require.Zero(t, bankKeeper.sendCoinsToModuleCalls)
	require.Zero(t, bankKeeper.burnCoinsCalls)
}

func TestPushNewActionsUserWithdrawalHappyPath(t *testing.T) {
	feePayer, cosmosPubKey, cosmosAddr := newUserMapping(t)

	action := bridgetypes.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  bridgetypes.ActionType_ACTION_TYPE_WITHDRAW,
		Amount:      7,
	}

	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 11,
		actions:         []bridgetypes.Action{action},
	}

	bankKeeper := NewMockBankKeeper()
	bankKeeper.spendable = sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 50))

	keyRegistryKeeper := NewMockKeyregistryKeeper()
	keyRegistryKeeper.register(feePayer, cosmosPubKey)

	f := initFixture(t, bankKeeper, client, keyRegistryKeeper)
	seedPushNewActionsState(t, f, 10)

	beforeRoot := latestActionsReducedRoot(t, f)

	ms := bridgekeeper.NewMsgServerImpl(f.keeper)
	resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 11,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	state, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(11), state.LatestFetchedMinaHeight)

	afterRoot := latestActionsReducedRoot(t, f)
	expectedCoins := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 7))

	require.NotEqual(t, beforeRoot, afterRoot)
	require.Equal(t, expectedActionsReducedRoot(t, action), afterRoot)

	require.Equal(t, 2, bankKeeper.spendableCalls)
	require.Equal(t, cosmosAddr, bankKeeper.lastSpendableAddr)

	require.Equal(t, 1, bankKeeper.sendCoinsToModuleCalls)
	require.Equal(t, cosmosAddr, bankKeeper.lastSendFromAddr)
	require.Equal(t, bridgetypes.ModuleName, bankKeeper.lastSendToModule)
	require.True(t, bankKeeper.lastSendToModuleCoins.Equal(expectedCoins))

	require.Equal(t, 1, bankKeeper.burnCoinsCalls)
	require.Equal(t, bridgetypes.ModuleName, bankKeeper.lastBurnModule)
	require.True(t, bankKeeper.lastBurnCoins.Equal(expectedCoins))

	require.Zero(t, bankKeeper.mintCoinsCalls)
	require.Zero(t, bankKeeper.sendCoinsFromModuleCalls)
}

func TestPushNewActionsRejectsNilArchiveWrapperClientWithoutMutatingState(t *testing.T) {
	f := initFixture(t, nil, nil, nil)
	seedPushNewActionsState(t, f, 10)

	beforeState, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)

	beforeRoot := latestActionsReducedRoot(t, f)

	ms := bridgekeeper.NewMsgServerImpl(f.keeper)
	resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 11,
	})

	require.ErrorIs(t, err, bridgetypes.ErrArchiveWrapperQueryClientNotConfigured)
	require.Nil(t, resp)

	afterState, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, beforeState, afterState)

	afterRoot := latestActionsReducedRoot(t, f)
	require.Equal(t, beforeRoot, afterRoot)
}

func TestPushNewActionsRollsBackStateOnBankKeeperErrors(t *testing.T) {
	tests := []struct {
		name                    string
		actionType              bridgetypes.ActionType
		configureBankKeeper     func(*mockBankKeeper)
		wantSpendableCalls      int
		wantMintCalls           int
		wantSendFromModuleCalls int
		wantSendToModuleCalls   int
		wantBurnCalls           int
	}{
		{
			name:       "deposit mint failure",
			actionType: bridgetypes.ActionType_ACTION_TYPE_DEPOSIT,
			configureBankKeeper: func(b *mockBankKeeper) {
				b.mintErr = errors.New("mint failed")
			},
			wantSpendableCalls:      0,
			wantMintCalls:           1,
			wantSendFromModuleCalls: 0,
			wantSendToModuleCalls:   0,
			wantBurnCalls:           0,
		},
		{
			name:       "deposit send failure",
			actionType: bridgetypes.ActionType_ACTION_TYPE_DEPOSIT,
			configureBankKeeper: func(b *mockBankKeeper) {
				b.sendFromModuleErr = errors.New("send from module failed")
			},
			wantSpendableCalls:      0,
			wantMintCalls:           1,
			wantSendFromModuleCalls: 1,
			wantSendToModuleCalls:   0,
			wantBurnCalls:           0,
		},
		{
			name:       "withdraw send failure",
			actionType: bridgetypes.ActionType_ACTION_TYPE_WITHDRAW,
			configureBankKeeper: func(b *mockBankKeeper) {
				b.spendable = sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 50))
				b.sendToModuleErr = errors.New("send to module failed")
			},
			wantSpendableCalls:      2,
			wantMintCalls:           0,
			wantSendFromModuleCalls: 0,
			wantSendToModuleCalls:   1,
			wantBurnCalls:           0,
		},
		{
			name:       "withdraw burn failure",
			actionType: bridgetypes.ActionType_ACTION_TYPE_WITHDRAW,
			configureBankKeeper: func(b *mockBankKeeper) {
				b.spendable = sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 50))
				b.burnErr = errors.New("burn failed")
			},
			wantSpendableCalls:      2,
			wantMintCalls:           0,
			wantSendFromModuleCalls: 0,
			wantSendToModuleCalls:   1,
			wantBurnCalls:           1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feePayer, cosmosPubKey, _ := newUserMapping(t)

			action := bridgetypes.Action{
				BlockHeight: 11,
				FeePayer:    feePayer,
				ActionType:  tt.actionType,
				Amount:      7,
			}

			client := &stubArchiveWrapperQueryClient{
				minaBlockHeight: 11,
				actions:         []bridgetypes.Action{action},
			}

			bankKeeper := NewMockBankKeeper()
			tt.configureBankKeeper(bankKeeper)

			keyRegistryKeeper := NewMockKeyregistryKeeper()
			keyRegistryKeeper.register(feePayer, cosmosPubKey)

			f := initFixture(t, bankKeeper, client, keyRegistryKeeper)
			seedPushNewActionsState(t, f, 10)

			beforeState, err := f.keeper.GetBridgeState(f.ctx)
			require.NoError(t, err)

			beforeRoot := latestActionsReducedRoot(t, f)
			beforeBalance := cloneCoins(bankKeeper.spendable)

			ms := bridgekeeper.NewMsgServerImpl(f.keeper)
			resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
				Creator:         authorityString(t, f.addressCodec),
				MinaBlockHeight: 11,
			})

			require.Error(t, err)
			require.Nil(t, resp)

			afterState, getErr := f.keeper.GetBridgeState(f.ctx)
			require.NoError(t, getErr)
			require.Equal(t, beforeState, afterState)

			afterRoot := latestActionsReducedRoot(t, f)
			require.Equal(t, beforeRoot, afterRoot)
			require.Equal(t, beforeBalance, bankKeeper.spendable)

			require.Equal(t, 1, client.getMinaBlockHeightCalls)
			require.Equal(t, 1, client.getActionsCalls)

			require.Equal(t, tt.wantSpendableCalls, bankKeeper.spendableCalls)
			require.Equal(t, tt.wantMintCalls, bankKeeper.mintCoinsCalls)
			require.Equal(t, tt.wantSendFromModuleCalls, bankKeeper.sendCoinsFromModuleCalls)
			require.Equal(t, tt.wantSendToModuleCalls, bankKeeper.sendCoinsToModuleCalls)
			require.Equal(t, tt.wantBurnCalls, bankKeeper.burnCoinsCalls)
		})
	}
}

func TestPushNewActionsSkipsUnknownDepositButAdvancesCursor(t *testing.T) {
	feePayer := mustMinaFeePayerBytes(t)

	action := bridgetypes.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  bridgetypes.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      7,
	}

	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 11,
		actions:         []bridgetypes.Action{action},
	}

	bankKeeper := NewMockBankKeeper()
	keyRegistryKeeper := NewMockKeyregistryKeeper()

	f := initFixture(t, bankKeeper, client, keyRegistryKeeper)
	seedPushNewActionsState(t, f, 10)

	beforeRoot := latestActionsReducedRoot(t, f)

	ms := bridgekeeper.NewMsgServerImpl(f.keeper)
	resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 11,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	state, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(11), state.LatestFetchedMinaHeight)

	afterRoot := latestActionsReducedRoot(t, f)
	require.Equal(t, beforeRoot, afterRoot)

	require.Zero(t, bankKeeper.spendableCalls)
	require.Zero(t, bankKeeper.mintCoinsCalls)
	require.Zero(t, bankKeeper.sendCoinsFromModuleCalls)
	require.Zero(t, bankKeeper.sendCoinsToModuleCalls)
	require.Zero(t, bankKeeper.burnCoinsCalls)
}

func TestPushNewActionsSkipsWithdrawalWithInsufficientBalanceButAdvancesCursor(t *testing.T) {
	feePayer, cosmosPubKey, cosmosAddr := newUserMapping(t)

	action := bridgetypes.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  bridgetypes.ActionType_ACTION_TYPE_WITHDRAW,
		Amount:      7,
	}

	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 11,
		actions:         []bridgetypes.Action{action},
	}

	bankKeeper := NewMockBankKeeper()
	bankKeeper.spendable = sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 3))

	keyRegistryKeeper := NewMockKeyregistryKeeper()
	keyRegistryKeeper.register(feePayer, cosmosPubKey)

	f := initFixture(t, bankKeeper, client, keyRegistryKeeper)
	seedPushNewActionsState(t, f, 10)

	beforeRoot := latestActionsReducedRoot(t, f)

	ms := bridgekeeper.NewMsgServerImpl(f.keeper)
	resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 11,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	state, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(11), state.LatestFetchedMinaHeight)

	afterRoot := latestActionsReducedRoot(t, f)
	require.Equal(t, beforeRoot, afterRoot)

	require.Equal(t, 1, bankKeeper.spendableCalls)
	require.Equal(t, cosmosAddr, bankKeeper.lastSpendableAddr)

	require.Zero(t, bankKeeper.mintCoinsCalls)
	require.Zero(t, bankKeeper.sendCoinsFromModuleCalls)
	require.Zero(t, bankKeeper.sendCoinsToModuleCalls)
	require.Zero(t, bankKeeper.burnCoinsCalls)
}

func TestPushNewActionsWithdrawalBalanceAboveUint64DoesNotPanic(t *testing.T) {
	feePayer, cosmosPubKey, cosmosAddr := newUserMapping(t)

	action := bridgetypes.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  bridgetypes.ActionType_ACTION_TYPE_WITHDRAW,
		Amount:      1,
	}

	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 11,
		actions:         []bridgetypes.Action{action},
	}

	// 18446744073709551616 = 2^64
	largeBalance, ok := math.NewIntFromString("18446744073709551616")
	require.True(t, ok)

	bankKeeper := NewMockBankKeeper()
	bankKeeper.spendable = sdk.NewCoins(
		sdk.NewCoin(sdk.DefaultBondDenom, largeBalance),
	)

	keyRegistryKeeper := NewMockKeyregistryKeeper()
	keyRegistryKeeper.register(feePayer, cosmosPubKey)

	f := initFixture(t, bankKeeper, client, keyRegistryKeeper)
	seedPushNewActionsState(t, f, 10)

	resp, err := bridgekeeper.NewMsgServerImpl(f.keeper).PushNewActions(
		f.ctx,
		&bridgetypes.MsgPushNewActions{
			Creator:         authorityString(t, f.addressCodec),
			MinaBlockHeight: 11,
		},
	)

	require.NoError(t, err)
	require.NotNil(t, resp)

	state, err := f.keeper.GetBridgeState(f.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(11), state.LatestFetchedMinaHeight)

	require.Equal(t, 2, bankKeeper.spendableCalls)
	require.Equal(t, cosmosAddr, bankKeeper.lastSpendableAddr)
	require.Equal(t, 1, bankKeeper.sendCoinsToModuleCalls)
	require.Equal(t, 1, bankKeeper.burnCoinsCalls)
	require.Zero(t, bankKeeper.mintCoinsCalls)
	require.Zero(t, bankKeeper.sendCoinsFromModuleCalls)
}

func TestPushNewActionsPreservesWrapperActionOrderingInRoot(t *testing.T) {
	feePayer, cosmosPubKey, _ := newUserMapping(t)

	action1 := bridgetypes.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  bridgetypes.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      5,
	}
	action2 := bridgetypes.Action{
		BlockHeight: 11,
		FeePayer:    feePayer,
		ActionType:  bridgetypes.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      9,
	}

	client := &stubArchiveWrapperQueryClient{
		minaBlockHeight: 11,
		actions:         []bridgetypes.Action{action1, action2},
	}

	bankKeeper := NewMockBankKeeper()
	keyRegistryKeeper := NewMockKeyregistryKeeper()
	keyRegistryKeeper.register(feePayer, cosmosPubKey)

	f := initFixture(t, bankKeeper, client, keyRegistryKeeper)
	seedPushNewActionsState(t, f, 10)

	ms := bridgekeeper.NewMsgServerImpl(f.keeper)
	resp, err := ms.PushNewActions(f.ctx, &bridgetypes.MsgPushNewActions{
		Creator:         authorityString(t, f.addressCodec),
		MinaBlockHeight: 11,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	gotRoot := latestActionsReducedRoot(t, f)
	forwardRoot := expectedActionsReducedRoot(t, action1, action2)
	reversedRoot := expectedActionsReducedRoot(t, action2, action1)

	require.Equal(t, forwardRoot, gotRoot)
	require.NotEqual(t, reversedRoot, gotRoot)

	require.Equal(t, 2, bankKeeper.mintCoinsCalls)
	require.Equal(t, 2, bankKeeper.sendCoinsFromModuleCalls)
	require.Zero(t, bankKeeper.sendCoinsToModuleCalls)
	require.Zero(t, bankKeeper.burnCoinsCalls)
}
