package app

import (
	"encoding/base64"
	"net"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	minafield "github.com/node101-io/mina-signer-go/field"
	bridgetypes "github.com/node101-io/pulsar-chain/x/bridge/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpcHealthV1 "google.golang.org/grpc/health/grpc_health_v1"
)

func startArchiveWrapperHealthServer(
	t *testing.T,
	status grpcHealthV1.HealthCheckResponse_ServingStatus,
) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("query.Query", status)
	grpcHealthV1.RegisterHealthServer(server, healthServer)

	go func() {
		_ = server.Serve(listener)
	}()

	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	return listener.Addr().String()
}

func TestAppConstructionDoesNotRequireReadyArchiveWrapper(t *testing.T) {
	wrapperAddress := startArchiveWrapperHealthServer(
		t,
		grpcHealthV1.HealthCheckResponse_NOT_SERVING,
	)

	app := newZeroHeightExportTestApp(t, wrapperAddress)
	require.NotNil(t, app.BridgeArchiveWrapperClient)
}

func newZeroHeightExportTestApp(t *testing.T, wrapperAddress string) *App {
	t.Helper()

	privateKey := make([]byte, 32)
	privateKey[0] = 1
	appOptions := simtestutil.AppOptionsMap{
		flags.FlagHome:                       t.TempDir(),
		"bridge.wrapper_grpc_address":        wrapperAddress,
		"bridge.wrapper_grpc_transport_mode": "loopback",
		"mina.network_id":                    string(mina.TestNet),
		"vote_extension.priv_key":            base64.StdEncoding.EncodeToString(privateKey),
	}

	app := New(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		appOptions,
		fauxMerkleModeOpt,
		baseapp.SetChainID(SimAppChainID),
	)
	t.Cleanup(func() {
		require.NoError(t, app.Close())
	})

	return app
}

func canonicalRoot(value uint64) []byte {
	return minafield.NewField().FromUint64(value).Bytes()
}

func bridgeGenesisFromExport(t *testing.T, app *App, appState []byte) bridgetypes.GenesisState {
	t.Helper()

	var exportedGenesis GenesisState
	require.NoError(t, cmtjson.Unmarshal(appState, &exportedGenesis))

	var bridgeGenesis bridgetypes.GenesisState
	app.AppCodec().MustUnmarshalJSON(
		exportedGenesis[bridgetypes.ModuleName],
		&bridgeGenesis,
	)

	return bridgeGenesis
}

func TestZeroHeightExportNormalizesBridgeRootSnapshots(t *testing.T) {
	wrapperAddress := startArchiveWrapperHealthServer(
		t,
		grpcHealthV1.HealthCheckResponse_NOT_SERVING,
	)
	oldApp := newZeroHeightExportTestApp(t, wrapperAddress)

	validatorSet, err := simtestutil.CreateRandomValidatorSet()
	require.NoError(t, err)

	accountKey := secp256k1.GenPrivKey()
	account := authtypes.NewBaseAccount(accountKey.PubKey().Address().Bytes(), accountKey.PubKey(), 0, 0)
	balance := banktypes.Balance{
		Address: account.GetAddress().String(),
		Coins: sdk.NewCoins(
			sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(100_000_000_000_000)),
		),
	}

	genesisState, err := simtestutil.GenesisStateWithValSet(
		oldApp.AppCodec(),
		oldApp.DefaultGenesis(),
		validatorSet,
		[]authtypes.GenesisAccount{account},
		balance,
	)
	require.NoError(t, err)

	const minaCursor int64 = 500_000
	bridgeGenesis := bridgetypes.DefaultGenesis()
	bridgeGenesis.Params = bridgetypes.DefaultTestParams()
	bridgeGenesis.BridgeState = bridgetypes.BridgeState{
		LatestFetchedMinaHeight:            minaCursor,
		StartMinaHeight:                    499_990,
		ValidActionHashesCosmosBlockHeight: 103,
		ValidActionHashes: []string{
			minafield.NewField().FromUint64(77).String(),
			minafield.NewField().FromUint64(78).String(),
		},
	}
	bridgeGenesis.ActionsReducedRootSnapshots = nil
	for height := int64(100); height <= 103; height++ {
		bridgeGenesis.ActionsReducedRootSnapshots = append(
			bridgeGenesis.ActionsReducedRootSnapshots,
			bridgetypes.ActionsReducedRootSnapshot{
				CosmosBlockHeight:  height,
				ActionsReducedRoot: canonicalRoot(uint64(height)),
			},
		)
	}
	genesisState[bridgetypes.ModuleName] = oldApp.AppCodec().MustMarshalJSON(bridgeGenesis)

	appState, err := cmtjson.Marshal(genesisState)
	require.NoError(t, err)
	_, err = oldApp.InitChain(&abci.RequestInitChain{
		ChainId:         SimAppChainID,
		InitialHeight:   1,
		ConsensusParams: simtestutil.DefaultConsensusParams,
		AppStateBytes:   appState,
	})
	require.NoError(t, err)
	oldApp.SimWriteState()
	_, err = oldApp.Commit()
	require.NoError(t, err)

	preExportCtx := oldApp.NewContextLegacy(true, cmtproto.Header{Height: oldApp.LastBlockHeight()})
	preExportRoot, err := oldApp.BridgeKeeper.GetLatestActionsReducedRoot(preExportCtx)
	require.NoError(t, err)
	require.Equal(t, canonicalRoot(103), preExportRoot)

	normalExport, err := oldApp.ExportAppStateAndValidators(false, nil, nil)
	require.NoError(t, err)
	normalBridgeGenesis := bridgeGenesisFromExport(t, oldApp, normalExport.AppState)
	require.Equal(t, bridgeGenesis.BridgeState, normalBridgeGenesis.BridgeState)
	require.Len(t, normalBridgeGenesis.ActionsReducedRootSnapshots, 4)
	require.Equal(t, []int64{100, 101, 102, 103}, []int64{
		normalBridgeGenesis.ActionsReducedRootSnapshots[0].CosmosBlockHeight,
		normalBridgeGenesis.ActionsReducedRootSnapshots[1].CosmosBlockHeight,
		normalBridgeGenesis.ActionsReducedRootSnapshots[2].CosmosBlockHeight,
		normalBridgeGenesis.ActionsReducedRootSnapshots[3].CosmosBlockHeight,
	})

	exported, err := oldApp.ExportAppStateAndValidators(true, nil, nil)
	require.NoError(t, err)
	require.Equal(t, int64(0), exported.Height)

	exportedBridgeGenesis := bridgeGenesisFromExport(t, oldApp, exported.AppState)
	require.Equal(t, minaCursor, exportedBridgeGenesis.BridgeState.LatestFetchedMinaHeight)
	require.Empty(t, exportedBridgeGenesis.BridgeState.ValidActionHashes)
	require.Equal(t, int64(0), exportedBridgeGenesis.BridgeState.ValidActionHashesCosmosBlockHeight)
	require.Equal(t, minaCursor, exportedBridgeGenesis.BridgeState.StartMinaHeight)
	require.Len(t, exportedBridgeGenesis.ActionsReducedRootSnapshots, 1)
	require.Equal(t, int64(0), exportedBridgeGenesis.ActionsReducedRootSnapshots[0].CosmosBlockHeight)
	require.Equal(t, canonicalRoot(103), exportedBridgeGenesis.ActionsReducedRootSnapshots[0].ActionsReducedRoot)

	newApp := newZeroHeightExportTestApp(t, wrapperAddress)
	_, err = newApp.InitChain(&abci.RequestInitChain{
		ChainId:         SimAppChainID,
		InitialHeight:   1,
		ConsensusParams: &exported.ConsensusParams,
		AppStateBytes:   exported.AppState,
	})
	require.NoError(t, err)
	newApp.SimWriteState()
	_, err = newApp.Commit()
	require.NoError(t, err)

	ctx := newApp.NewContextLegacy(true, cmtproto.Header{Height: 0})
	rootAtZero, err := newApp.BridgeKeeper.GetActionsReducedRootAtHeight(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, canonicalRoot(103), rootAtZero)

	require.NoError(t, newApp.BridgeKeeper.SetActionsReducedRoot(ctx, 1, canonicalRoot(201)))
	latestRoot, err := newApp.BridgeKeeper.GetLatestActionsReducedRoot(ctx)
	require.NoError(t, err)
	require.Equal(t, canonicalRoot(201), latestRoot)

	importedBridgeState, err := newApp.BridgeKeeper.GetBridgeState(ctx)
	require.NoError(t, err)
	require.Equal(t, minaCursor, importedBridgeState.LatestFetchedMinaHeight)
	require.Empty(t, importedBridgeState.ValidActionHashes)
	require.Equal(t, int64(0), importedBridgeState.ValidActionHashesCosmosBlockHeight)
	require.Equal(t, minaCursor, importedBridgeState.StartMinaHeight)
}
