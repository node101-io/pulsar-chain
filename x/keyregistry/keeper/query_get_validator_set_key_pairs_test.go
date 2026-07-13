package keeper_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	module "github.com/node101-io/pulsar-chain/x/keyregistry/module"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type validatorSetTestStakingKeeper struct {
	validators     []stakingtypes.ValidatorI
	historicalInfo map[int64]stakingtypes.HistoricalInfo
}

func (k *validatorSetTestStakingKeeper) IterateLastValidators(_ context.Context, fn func(int64, stakingtypes.ValidatorI) bool) error {
	for i, validator := range k.validators {
		if fn(int64(i), validator) {
			break
		}
	}

	return nil
}

func (k *validatorSetTestStakingKeeper) GetHistoricalInfo(_ context.Context, height int64) (stakingtypes.HistoricalInfo, error) {
	historicalInfo, ok := k.historicalInfo[height]
	if !ok {
		return stakingtypes.HistoricalInfo{}, fmt.Errorf("historical info not found")
	}

	return historicalInfo, nil
}

func (*validatorSetTestStakingKeeper) GetValidatorByConsAddr(context.Context, sdk.ConsAddress) (stakingtypes.Validator, error) {
	return stakingtypes.Validator{}, fmt.Errorf("unexpected GetValidatorByConsAddr call")
}

func TestValidatorSetInvalidArgumentFail(t *testing.T) {
	f := initValidatorSetFixture(t, &validatorSetTestStakingKeeper{}, 10)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetValidatorSetByHeight(f.ctx, nil)
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestValidatorSetNegativeHeight(t *testing.T) {
	f := initValidatorSetFixture(t, &validatorSetTestStakingKeeper{}, 10)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.GetValidatorSetByHeight(f.ctx, &types.QueryGetValidatorSetByHeightRequest{
		BlockHeight: 0,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "negative block height")
}

func TestValidatorSetCurrentHeightSuccess(t *testing.T) {
	stakingKeeper := &validatorSetTestStakingKeeper{}
	f := initValidatorSetFixture(t, stakingKeeper, 12)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	ms := keeper.NewMsgServerImpl(f.keeper)

	lowPowerValidator, lowPowerCosmosKey, lowPowerMinaKey := registerValidatorSetEntry(t, f, ms, 5, 1)
	highPowerValidatorA, highPowerCosmosKeyA, highPowerMinaKeyA := registerValidatorSetEntry(t, f, ms, 10, 2)
	highPowerValidatorB, highPowerCosmosKeyB, highPowerMinaKeyB := registerValidatorSetEntry(t, f, ms, 10, 3)

	stakingKeeper.validators = []stakingtypes.ValidatorI{
		lowPowerValidator,
		highPowerValidatorB,
		highPowerValidatorA,
	}

	expected := []*types.ValidatorSetEntry{
		{
			ValidatorCosmosPubKey: highPowerCosmosKeyA,
			ValidatorMinaPubKey:   highPowerMinaKeyA,
			ConsensusPower:        10,
		},
		{
			ValidatorCosmosPubKey: highPowerCosmosKeyB,
			ValidatorMinaPubKey:   highPowerMinaKeyB,
			ConsensusPower:        10,
		},
		{
			ValidatorCosmosPubKey: lowPowerCosmosKey,
			ValidatorMinaPubKey:   lowPowerMinaKey,
			ConsensusPower:        5,
		},
	}

	firstConsAddr, err := highPowerValidatorA.GetConsAddr()
	require.NoError(t, err)
	secondConsAddr, err := highPowerValidatorB.GetConsAddr()
	require.NoError(t, err)

	if bytes.Compare(firstConsAddr, secondConsAddr) > 0 {
		expected[0], expected[1] = expected[1], expected[0]
	}

	queryResp, err := qs.GetValidatorSetByHeight(f.ctx, &types.QueryGetValidatorSetByHeightRequest{
		BlockHeight: 12,
	})
	require.NoError(t, err)
	require.NotNil(t, queryResp)
	require.Equal(t, expected, queryResp.Validators)
}

func TestValidatorSetHistoricalHeightSuccess(t *testing.T) {
	stakingKeeper := &validatorSetTestStakingKeeper{}
	f := initValidatorSetFixture(t, stakingKeeper, 12)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	ms := keeper.NewMsgServerImpl(f.keeper)

	lowPowerValidator, lowPowerCosmosKey, lowPowerMinaKey := registerValidatorSetEntry(t, f, ms, 3, 4)
	highPowerValidator, highPowerCosmosKey, highPowerMinaKey := registerValidatorSetEntry(t, f, ms, 9, 5)

	stakingKeeper.historicalInfo = map[int64]stakingtypes.HistoricalInfo{
		7: {
			Valset: []stakingtypes.Validator{
				lowPowerValidator,
				highPowerValidator,
			},
		},
	}

	queryResp, err := qs.GetValidatorSetByHeight(f.ctx, &types.QueryGetValidatorSetByHeightRequest{
		BlockHeight: 7,
	})
	require.NoError(t, err)
	require.NotNil(t, queryResp)
	require.Equal(t, []*types.ValidatorSetEntry{
		{
			ValidatorCosmosPubKey: highPowerCosmosKey,
			ValidatorMinaPubKey:   highPowerMinaKey,
			ConsensusPower:        9,
		},
		{
			ValidatorCosmosPubKey: lowPowerCosmosKey,
			ValidatorMinaPubKey:   lowPowerMinaKey,
			ConsensusPower:        3,
		},
	}, queryResp.Validators)
}

func initValidatorSetFixture(t *testing.T, stakingKeeper types.StakingKeeper, blockHeight int64) *fixture {
	t.Helper()

	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx.WithBlockHeight(blockHeight)
	authority := authtypes.NewModuleAddress(types.GovModuleName)

	k := keeper.NewKeeper(
		storeService,
		encCfg.Codec,
		addressCodec,
		stakingKeeper,
		authority,
	)

	return &fixture{
		ctx:          ctx,
		keeper:       k,
		addressCodec: addressCodec,
	}
}

func registerValidatorSetEntry(
	t *testing.T,
	f *fixture,
	ms types.MsgServer,
	power int64,
	minaSeed byte,
) (stakingtypes.Validator, []byte, []byte) {
	t.Helper()

	cosmosPriv := generateValidatorCosmosPrivKey()
	minaPriv := newValidatorSetMinaKey(t, minaSeed)

	creator, cosmosPubKey, minaPubKey, cosmosSig, minaSig, err := signValidatorRegistration(cosmosPriv, minaPriv)
	require.NoError(t, err)

	resp, err := ms.RegisterKeys(f.ctx, &types.MsgRegisterKeys{
		Creator:         creator,
		CosmosSignature: cosmosSig,
		MinaSignature:   minaSig,
		CosmosPublicKey: cosmosPubKey,
		MinaPublicKey:   minaPubKey,
		ActorType:       types.ActorType_VALIDATOR,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	return newValidatorSetValidator(t, cosmosPriv.PubKey(), power), cosmosPubKey, minaPubKey
}

func newValidatorSetValidator(t *testing.T, pubKey cmtcrypto.PubKey, power int64) stakingtypes.Validator {
	t.Helper()

	sdkPubKey, err := cryptocodec.FromCmtPubKeyInterface(pubKey)
	require.NoError(t, err)

	validator, err := stakingtypes.NewValidator(
		sdk.ValAddress(pubKey.Address()).String(),
		sdkPubKey,
		stakingtypes.Description{},
	)
	require.NoError(t, err)

	validator = validator.UpdateStatus(stakingtypes.Bonded)
	validator.Tokens = sdk.TokensFromConsensusPower(power, sdk.DefaultPowerReduction)

	return validator
}

func newValidatorSetMinaKey(t *testing.T, seed byte) *privatekey.PrivateKey {
	t.Helper()

	var seedBytes [32]byte
	for i := range seedBytes {
		seedBytes[i] = seed
	}

	minaPrivKey, err := privatekey.NewPrivateKeyFromBytes(seedBytes, mina.NetworkID(types.ActorType_VALIDATOR.String()))
	require.NoError(t, err)

	return minaPrivKey
}
