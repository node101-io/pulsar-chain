package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	clienthelpers "cosmossdk.io/client/v2/helpers"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	consensuskeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	gogogrpc "github.com/cosmos/gogoproto/grpc"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/controller/keeper"
	icahostkeeper "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/host/keeper"
	ibctransferkeeper "github.com/cosmos/ibc-go/v10/modules/apps/transfer/keeper"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"

	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/node101-io/mina-signer-go/privatekey"
	abcihandler "github.com/node101-io/pulsar-chain/abci"
	appante "github.com/node101-io/pulsar-chain/app/ante"
	"github.com/node101-io/pulsar-chain/docs"
	bridge "github.com/node101-io/pulsar-chain/x/bridge/keeper"
	keyregistrymodulekeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	verificationmodulekeeper "github.com/node101-io/pulsar-chain/x/verification/keeper"
	verificationsidecar "github.com/node101-io/pulsar-chain/x/verification/sidecar"
	votepersistencemodulekeeper "github.com/node101-io/pulsar-chain/x/votepersistence/keeper"
	"google.golang.org/grpc/health"
	grpcHealthV1 "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	// Name is the name of the application.
	Name = "pulsar"
	// AccountAddressPrefix is the prefix for accounts addresses.
	AccountAddressPrefix = "pulsar"
	// ChainCoinType is the coin type of the chain.
	ChainCoinType = 118
)

// DefaultNodeHome default home directories for the application daemon
var DefaultNodeHome string

var (
	_ runtime.AppI            = (*App)(nil)
	_ servertypes.Application = (*App)(nil)
)

// App extends an ABCI application, but with most of its parameters exported.
// They are exported for convenience in creating helper functions, as object
// capabilities aren't needed for testing.
type App struct {
	*runtime.App
	legacyAmino       *codec.LegacyAmino
	appCodec          codec.Codec
	txConfig          client.TxConfig
	interfaceRegistry codectypes.InterfaceRegistry

	// keepers
	// only keepers required by the app are exposed
	// the list of all modules is available in the app_config
	AuthKeeper            authkeeper.AccountKeeper
	BankKeeper            bankkeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        slashingkeeper.Keeper
	MintKeeper            mintkeeper.Keeper
	DistrKeeper           distrkeeper.Keeper
	GovKeeper             *govkeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	AuthzKeeper           authzkeeper.Keeper
	ConsensusParamsKeeper consensuskeeper.Keeper
	CircuitBreakerKeeper  circuitkeeper.Keeper
	FeeGrantKeeper        feegrantkeeper.Keeper
	ParamsKeeper          paramskeeper.Keeper //nolint:staticcheck // Legacy params keeper is still required for IBC params migration.

	// ibc keepers
	IBCKeeper           *ibckeeper.Keeper
	ICAControllerKeeper icacontrollerkeeper.Keeper
	ICAHostKeeper       icahostkeeper.Keeper
	TransferKeeper      ibctransferkeeper.Keeper

	// simulation manager
	sm                    *module.SimulationManager
	KeyregistryKeeper     keyregistrymodulekeeper.Keeper
	VotepersistenceKeeper votepersistencemodulekeeper.Keeper
	BridgeKeeper          bridge.Keeper
	VerificationKeeper    verificationmodulekeeper.Keeper

	ABCIHandler                *abcihandler.ABCIHandler
	BridgeArchiveWrapperClient *bridge.ArchiveWrapperClient
	// VerificationSidecarClient is non-nil only when the app created the gRPC
	// client itself; retaining it here gives App.Close lifecycle ownership.
	VerificationSidecarClient *verificationsidecar.Client
}

// RegisterGRPCServerWithSkipCheckHeader registers application and standard health services.
func (app *App) RegisterGRPCServerWithSkipCheckHeader(server gogogrpc.Server, skipCheckHeader bool) {
	app.App.RegisterGRPCServerWithSkipCheckHeader(server, skipCheckHeader)
	grpcHealthV1.RegisterHealthServer(server, health.NewServer())
}

func init() {
	var err error
	clienthelpers.EnvPrefix = Name
	DefaultNodeHome, err = clienthelpers.GetNodeHomeDirectory("." + Name)
	if err != nil {
		panic(err)
	}
}

// AppConfig returns the default app config.
func AppConfig() depinject.Config {
	return depinject.Configs(
		appConfig,
		depinject.Supply(
			// supply custom module basics
			map[string]module.AppModuleBasic{
				genutiltypes.ModuleName: genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
			},
		),
	)
}

// New returns a reference to an initialized App.
func New(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) *App {
	var (
		app        = &App{}
		appBuilder *runtime.AppBuilder

		// merge the AppConfig and other configuration in one config
		appConfig = depinject.Configs(
			AppConfig(),
			depinject.Supply(
				appOpts, // supply app options
				logger,  // supply logger

				// Supply with IBC keeper getter for the IBC modules with App Wiring.
				// The IBC Keeper cannot be passed because it has not been initiated yet.
				// Passing the getter, the app IBC Keeper will always be accessible.
				// This needs to be removed after IBC supports App Wiring.
				app.GetIBCKeeper,

				// here alternative options can be supplied to the DI container.
				// those options can be used f.e to override the default behavior of some modules.
				// for instance supplying a custom address codec for not using bech32 addresses.
				// read the depinject documentation and depinject module wiring for more information
				// on available options and how to use them.
			),
		)
	)

	var appModules map[string]appmodule.AppModule
	if err := depinject.Inject(appConfig,
		&appBuilder,
		&appModules,
		&app.appCodec,
		&app.legacyAmino,
		&app.txConfig,
		&app.interfaceRegistry,
		&app.AuthKeeper,
		&app.BankKeeper,
		&app.StakingKeeper,
		&app.SlashingKeeper,
		&app.MintKeeper,
		&app.DistrKeeper,
		&app.GovKeeper,
		&app.UpgradeKeeper,
		&app.AuthzKeeper,
		&app.ConsensusParamsKeeper,
		&app.CircuitBreakerKeeper,
		&app.FeeGrantKeeper,
		&app.ParamsKeeper,
		&app.KeyregistryKeeper,
		&app.VotepersistenceKeeper,
		&app.BridgeKeeper,
		&app.VerificationKeeper,
		&app.BridgeArchiveWrapperClient,
	); err != nil {
		panic(err)
	}

	networkId, ok := appOpts.Get("mina.network_id").(string)
	if !ok || networkId == "" {
		panic("mina.network_id is missing or not a string")
	}
	minaNetworkID, err := normalizeMinaNetworkID(networkId)
	if err != nil {
		panic(err)
	}

	secondaryKey, err := parseSecondaryKey(appOpts, minaNetworkID)
	if err != nil {
		panic(fmt.Sprintf("failed to parse vote extension secondary key: %v", err))
	}

	// Verification is structurally present in every ABCI handler. Validators use
	// the production builder by default, while explicit opt-outs and full nodes
	// receive a concrete no-op builder. Runtime sidecar outages remain non-fatal:
	// the gRPC connection is lazy and a missing result is omitted, never converted
	// into an invalid proof vote or allowed to suppress the mandatory Mina payload.
	selectedVerificationProvider, verificationClient, verificationActive, verificationConfigErr := verificationProvider(appOpts)
	if verificationConfigErr != nil {
		panic(fmt.Sprintf("invalid verification sidecar configuration: %v", verificationConfigErr))
	}
	verificationTimeout, verificationConfigErr := verificationTimeout(appOpts, verificationActive)
	if verificationConfigErr != nil {
		_ = verificationClient.Close()
		panic(fmt.Sprintf("invalid verification sidecar configuration: %v", verificationConfigErr))
	}
	verificationBuilder, verificationBuilderErr := newVerificationBuilder(
		appOpts,
		app.VerificationKeeper,
		selectedVerificationProvider,
		verificationActive,
		verificationTimeout,
	)
	if verificationBuilderErr != nil {
		// Enabled validators cannot safely continue with an unusable journal: they
		// could sign a commitment and later lose the salts required to reveal it.
		// This local initialization error is fail-fast; a sidecar that becomes
		// unavailable after startup remains a non-blocking runtime condition.
		_ = verificationClient.Close()
		panic(fmt.Sprintf("failed to initialize verification runtime: %v", verificationBuilderErr))
	}

	app.ABCIHandler, err = abcihandler.NewABCIHandler(
		secondaryKey,
		app.StakingKeeper,
		app.KeyregistryKeeper,
		app.VotepersistenceKeeper,
		minaNetworkID,
		app.BridgeKeeper,
		app.VerificationKeeper,
		verificationBuilder,
	)
	if err != nil {
		_ = verificationClient.Close()
		panic(fmt.Sprintf("failed to initialize ABCI handler: %v", err))
	}
	app.VerificationSidecarClient = verificationClient

	appante.RegisterInterfaces(app.interfaceRegistry)

	// add to default baseapp options
	// enable optimistic execution
	baseAppOptions = append(
		baseAppOptions,
		baseapp.SetOptimisticExecution(),
		func(bApp *baseapp.BaseApp) {
			anteHandler, err := appante.NewAnteHandler(appante.HandlerOptions{
				AccountKeeper:     app.AuthKeeper,
				BankKeeper:        app.BankKeeper,
				FeegrantKeeper:    app.FeeGrantKeeper,
				SignModeHandler:   app.txConfig.SignModeHandler(),
				SigGasConsumer:    authante.DefaultSigVerificationGasConsumer,
				KeyregistryKeeper: &app.KeyregistryKeeper,
				MinaNetworkID:     string(minaNetworkID),
				Logger:            logger,
			})
			if err != nil {
				panic(err)
			}

			bApp.SetAnteHandler(anteHandler)
		},
	)

	// build app
	app.App = appBuilder.Build(db, traceStore, baseAppOptions...)

	app.SetExtendVoteHandler(app.ABCIHandler.ExtendVoteHandler())
	app.SetVerifyVoteExtensionHandler(app.ABCIHandler.VerifyVoteExtensionHandler())
	app.SetPrepareProposal(app.ABCIHandler.PrepareProposalHandler())
	app.SetProcessProposal(app.ABCIHandler.ProcessProposalHandler())
	// Compose the SDK runtime and custom vote-extension work in one cache. This
	// makes Mina persistence, verification actions, module pre-block changes, and
	// the validator snapshot one atomic state transition: any failure rolls back
	// all of them instead of committing a partially interpreted proposal.
	runtimePreBlocker := app.App.PreBlocker
	verificationPreBlocker := app.ABCIHandler.PreBlocker()
	app.SetPreBlocker(func(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
		cacheCtx, write := ctx.CacheContext()
		response, err := runtimePreBlocker(cacheCtx, req)
		if err != nil {
			return nil, err
		}
		if _, err := verificationPreBlocker(cacheCtx, req); err != nil {
			return nil, err
		}
		write()
		return response, nil
	})
	abcihandler.RegisterQueryServer(app.GRPCQueryRouter(), app.ABCIHandler)

	// register legacy modules
	if err := app.registerIBCModules(appOpts); err != nil {
		panic(err)
	}

	/****  Module Options ****/

	// create the simulation manager and define the order of the modules for deterministic simulations
	overrideModules := map[string]module.AppModuleSimulation{
		authtypes.ModuleName: auth.NewAppModule(app.appCodec, app.AuthKeeper, authsims.RandomGenesisAccounts, nil),
	}
	app.sm = module.NewSimulationManagerFromAppModules(app.ModuleManager.Modules, overrideModules)

	app.sm.RegisterStoreDecoders()

	// A custom InitChainer sets if extra pre-init-genesis logic is required.
	// This is necessary for manually registered modules that do not support app wiring.
	// Manually set the module version map as shown below.
	// The upgrade module will automatically handle de-duplication of the module version map.
	app.SetInitChainer(func(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
		if err := app.UpgradeKeeper.SetModuleVersionMap(ctx, app.ModuleManager.GetVersionMap()); err != nil {
			return nil, err
		}
		return app.App.InitChainer(ctx, req)
	})

	if err := app.Load(loadLatest); err != nil {
		panic(err)
	}

	return app
}

func (app *App) Close() error {
	var errs []error

	if app.BridgeArchiveWrapperClient != nil {
		if err := app.BridgeArchiveWrapperClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if app.VerificationSidecarClient != nil {
		if err := app.VerificationSidecarClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if app.App != nil {
		if err := app.App.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// GetSubspace returns a param subspace for a given module name.
func (app *App) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

// LegacyAmino returns App's amino codec.
func (app *App) LegacyAmino() *codec.LegacyAmino {
	return app.legacyAmino
}

// AppCodec returns App's app codec.
func (app *App) AppCodec() codec.Codec {
	return app.appCodec
}

// InterfaceRegistry returns App's InterfaceRegistry.
func (app *App) InterfaceRegistry() codectypes.InterfaceRegistry {
	return app.interfaceRegistry
}

// TxConfig returns App's TxConfig
func (app *App) TxConfig() client.TxConfig {
	return app.txConfig
}

// GetKey returns the KVStoreKey for the provided store key.
func (app *App) GetKey(storeKey string) *storetypes.KVStoreKey {
	kvStoreKey, ok := app.UnsafeFindStoreKey(storeKey).(*storetypes.KVStoreKey)
	if !ok {
		return nil
	}
	return kvStoreKey
}

// SimulationManager implements the SimulationApp interface
func (app *App) SimulationManager() *module.SimulationManager {
	return app.sm
}

// RegisterAPIRoutes registers all application module routes with the provided
// API server.
func (app *App) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	app.App.RegisterAPIRoutes(apiSvr, apiConfig)
	if err := abcihandler.RegisterQueryHandlerClient(
		apiSvr.ClientCtx.CmdContext,
		apiSvr.GRPCGatewayRouter,
		abcihandler.NewQueryClient(apiSvr.ClientCtx),
	); err != nil {
		panic(err)
	}

	// register swagger API in app.go so that other applications can override easily
	if err := server.RegisterSwaggerAPI(apiSvr.ClientCtx, apiSvr.Router, apiConfig.Swagger); err != nil {
		panic(err)
	}

	// register app's OpenAPI routes.
	docs.RegisterOpenAPIService(Name, apiSvr.Router)
}

func parseSecondaryKey(appOpts servertypes.AppOptions, networkID mina.NetworkID) (abcihandler.SecondaryKey, error) {

	minaPrivKey := appOpts.Get("vote_extension.priv_key")
	keyStr, ok := minaPrivKey.(string)
	if !ok {
		return abcihandler.SecondaryKey{}, fmt.Errorf("vote_extension.priv_key is not a string")
	}

	keyBytes, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return abcihandler.SecondaryKey{}, fmt.Errorf("decode base64 private key: %w", err)
	}
	if len(keyBytes) != privatekey.Size() {
		return abcihandler.SecondaryKey{}, fmt.Errorf("invalid private key length: got %d bytes, want %d", len(keyBytes), privatekey.Size())
	}

	var rawPrivateKey [32]byte
	copy(rawPrivateKey[:], keyBytes)

	priv, err := privatekey.NewPrivateKeyFromBytes(rawPrivateKey, networkID)
	if err != nil {
		return abcihandler.SecondaryKey{}, fmt.Errorf("parse mina private key: %w", err)
	}
	public, err := priv.ToPublicKey()
	if err != nil {
		return abcihandler.SecondaryKey{}, fmt.Errorf("derive mina public key: %w", err)
	}

	return abcihandler.SecondaryKey{
		SecretKey: priv,
		PublicKey: public,
	}, nil
}

// GetMaccPerms returns a copy of the module account permissions
//
// NOTE: This is solely to be used for testing purposes.
func GetMaccPerms() map[string][]string {
	dup := make(map[string][]string)
	for _, perms := range moduleAccPerms {
		dup[perms.GetAccount()] = perms.GetPermissions()
	}

	return dup
}

// BlockedAddresses returns all the app's blocked account addresses.
func BlockedAddresses() map[string]bool {
	result := make(map[string]bool)

	if len(blockAccAddrs) > 0 {
		for _, addr := range blockAccAddrs {
			result[addr] = true
		}
	} else {
		for addr := range GetMaccPerms() {
			result[addr] = true
		}
	}

	return result
}
