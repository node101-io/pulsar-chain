package verification

import (
	"cosmossdk.io/core/address"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/store"
	"cosmossdk.io/depinject"
	"cosmossdk.io/depinject/appconfig"
	"github.com/cosmos/cosmos-sdk/codec"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"

	"github.com/node101-io/pulsar-chain/x/verification/keeper"
	"github.com/node101-io/pulsar-chain/x/verification/types"
)

var _ depinject.OnePerModuleType = AppModule{}

// IsOnePerModuleType prevents duplicate verification module instances.
func (AppModule) IsOnePerModuleType() {}

func init() {
	appconfig.Register(&types.Module{}, appconfig.Provide(ProvideModule))
}

// ModuleInputs declares the replicated dependencies required to construct the
// verification keeper. The sidecar client and local journal are absent because
// they belong to app/ABCI wiring, not Cosmos module dependency injection.
type ModuleInputs struct {
	depinject.In

	Config        *types.Module
	StoreService  store.KVStoreService
	Codec         codec.Codec
	AddressCodec  address.Codec
	StakingKeeper *stakingkeeper.Keeper
}

// ModuleOutputs exposes the keeper to ABCI/app wiring and the AppModule to the
// SDK runtime. This keeps one authoritative consensus keeper while allowing each
// validator process to attach its own optional result provider.
type ModuleOutputs struct {
	depinject.Out

	VerificationKeeper keeper.Keeper
	Module             appmodule.AppModule
}

// ProvideModule constructs the keeper with governance authority and registers
// it as a Cosmos SDK application module. Governance may change bounded module
// parameters, while validator-generated commitment actions remain outside the
// public Msg service.
func ProvideModule(in ModuleInputs) ModuleOutputs {
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	if in.Config.Authority != "" {
		authority = authtypes.NewModuleAddressOrBech32Address(in.Config.Authority)
	}
	k := keeper.NewKeeper(in.StoreService, in.Codec, in.AddressCodec, authority, in.StakingKeeper)
	m := NewAppModule(in.Codec, k)

	return ModuleOutputs{VerificationKeeper: k, Module: m}
}
