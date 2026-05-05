package keeper_test

import (
	"testing"
	"time"

	"cosmossdk.io/core/address"
	storetypes "cosmossdk.io/store/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	staking "github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtestutil "github.com/cosmos/cosmos-sdk/x/staking/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"go.uber.org/mock/gomock"

	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	keyregistrymodule "github.com/node101-io/pulsar-chain/x/keyregistry/module"
	keyregistrytypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/node101-io/pulsar-chain/x/votepersistence/keeper"
	module "github.com/node101-io/pulsar-chain/x/votepersistence/module"
	"github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

var (
	bondedAcc    = authtypes.NewEmptyModuleAccount(stakingtypes.BondedPoolName)
	notBondedAcc = authtypes.NewEmptyModuleAccount(stakingtypes.NotBondedPoolName)
)

type fixture struct {
	ctx               sdk.Context
	keeper            keeper.Keeper
	keyregistryKeeper keyregistrykeeper.Keeper
	stakingKeeper     *stakingkeeper.Keeper
	addressCodec      address.Codec
}

func initFixture(t *testing.T) *fixture {
	t.Helper()

	encCfg := moduletestutil.MakeTestEncodingConfig(
		module.AppModule{},
		keyregistrymodule.AppModule{},
		staking.AppModuleBasic{},
	)
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	validatorAddressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix())
	consensusAddressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32ConsensusAddrPrefix())

	storeKeys := storetypes.NewKVStoreKeys(types.StoreKey, keyregistrytypes.StoreKey, stakingtypes.StoreKey)
	transientKeys := storetypes.NewTransientStoreKeys("transient_test")
	ctx := testutil.DefaultContextWithKeys(storeKeys, transientKeys, nil).WithBlockHeader(tmproto.Header{
		Time: time.Now(),
	})

	authority := authtypes.NewModuleAddress(types.GovModuleName)

	ctrl := gomock.NewController(t)
	accountKeeper := stakingtestutil.NewMockAccountKeeper(ctrl)
	accountKeeper.EXPECT().GetModuleAddress(stakingtypes.BondedPoolName).Return(bondedAcc.GetAddress()).AnyTimes()
	accountKeeper.EXPECT().GetModuleAddress(stakingtypes.NotBondedPoolName).Return(notBondedAcc.GetAddress()).AnyTimes()
	accountKeeper.EXPECT().AddressCodec().Return(addressCodec).AnyTimes()

	bankKeeper := stakingtestutil.NewMockBankKeeper(ctrl)

	stakingStoreService := runtime.NewKVStoreService(storeKeys[stakingtypes.StoreKey])
	stakingKeeper := stakingkeeper.NewKeeper(
		encCfg.Codec,
		stakingStoreService,
		accountKeeper,
		bankKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		validatorAddressCodec,
		consensusAddressCodec,
	)
	if err := stakingKeeper.SetParams(ctx, stakingtypes.DefaultParams()); err != nil {
		t.Fatalf("failed to set staking params: %v", err)
	}

	keyregistryStoreService := runtime.NewKVStoreService(storeKeys[keyregistrytypes.StoreKey])
	keyregistryKeeper := keyregistrykeeper.NewKeeper(
		keyregistryStoreService,
		encCfg.Codec,
		addressCodec,
		authority,
	)
	if err := keyregistryKeeper.Params.Set(ctx, keyregistrytypes.DefaultParams()); err != nil {
		t.Fatalf("failed to set keyregistry params: %v", err)
	}

	votepersistenceStoreService := runtime.NewKVStoreService(storeKeys[types.StoreKey])
	k := keeper.NewKeeper(
		votepersistenceStoreService,
		encCfg.Codec,
		addressCodec,
		stakingKeeper,
		keyregistryKeeper,
		authority,
	)

	// Initialize params
	if err := k.Params.Set(ctx, types.DefaultParams()); err != nil {
		t.Fatalf("failed to set params: %v", err)
	}

	return &fixture{
		ctx:               ctx,
		keeper:            k,
		keyregistryKeeper: keyregistryKeeper,
		stakingKeeper:     stakingKeeper,
		addressCodec:      addressCodec,
	}
}
