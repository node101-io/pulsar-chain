package bridge

import (
	"strings"

	"cosmossdk.io/core/address"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/depinject/appconfig"
	"github.com/cosmos/cosmos-sdk/codec"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	wrapperquery "github.com/node101-io/archive-wrapper/query"

	"github.com/node101-io/pulsar-chain/x/bridge/keeper"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

var _ depinject.OnePerModuleType = AppModule{}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() {}

func init() {
	appconfig.Register(
		&types.Module{},
		appconfig.Provide(ProvideModule),
	)
}

type ModuleInputs struct {
	depinject.In

	Config       *types.Module
	AppOpts      servertypes.AppOptions `optional:"true"`
	StoreService store.KVStoreService
	Cdc          codec.Codec
	AddressCodec address.Codec

	AuthKeeper        types.AuthKeeper
	BankKeeper        types.BankKeeper
	KeyregistryKeeper types.KeyregistryKeeper
}

type ModuleOutputs struct {
	depinject.Out

	BridgeKeeper keeper.Keeper
	Module       appmodule.AppModule
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	// default to governance authority if not provided
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	if in.Config.Authority != "" {
		authority = authtypes.NewModuleAddressOrBech32Address(in.Config.Authority)
	}

	var archiveWrapperClient wrapperquery.QueryClient
	if in.AppOpts != nil {
		wrapperGRPCAddress, _ := in.AppOpts.Get("bridge.wrapper_grpc_address").(string)
		if strings.TrimSpace(wrapperGRPCAddress) == "" {
			panic("bridge.wrapper_grpc_address must be set in app.toml")
		}

		var err error
		archiveWrapperClient, err = keeper.NewArchiveWrapperQueryClient(wrapperGRPCAddress)
		if err != nil {
			panic(err)
		}
	}

	k := keeper.NewKeeper(
		in.StoreService,
		in.Cdc,
		in.AddressCodec,
		authority,
		in.BankKeeper,
		in.KeyregistryKeeper,
		archiveWrapperClient,
	)
	m := NewAppModule(in.Cdc, k, in.AuthKeeper, in.BankKeeper)

	return ModuleOutputs{
		BridgeKeeper: k,
		Module:       m,
	}
}
