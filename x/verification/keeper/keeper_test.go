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
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/verification/keeper"
	verificationmodule "github.com/node101-io/pulsar-chain/x/verification/module"
	"github.com/node101-io/pulsar-chain/x/verification/types"
)

type stakingKeeper struct {
	validatorCodec address.Codec
	lastValidators []stakingtypes.Validator
}

func (s *stakingKeeper) GetLastValidators(context.Context) ([]stakingtypes.Validator, error) {
	return append([]stakingtypes.Validator(nil), s.lastValidators...), nil
}

func (s *stakingKeeper) ValidatorAddressCodec() address.Codec {
	return s.validatorCodec
}

type validatorIdentity struct {
	account   []byte
	signer    string
	operator  []byte
	operatorS string
}

type fixture struct {
	ctx          sdk.Context
	keeper       keeper.Keeper
	msgServer    types.MsgServer
	query        types.QueryServer
	staking      *stakingKeeper
	accountCodec address.Codec
	validators   []validatorIdentity
}

func initFixture(t testing.TB, validatorCount int) *fixture {
	t.Helper()
	encCfg := moduletestutil.MakeTestEncodingConfig(verificationmodule.AppModule{})
	accountCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	validatorCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_verification")).Ctx

	staking := &stakingKeeper{validatorCodec: validatorCodec}
	identities := make([]validatorIdentity, 0, validatorCount)
	for i := 0; i < validatorCount; i++ {
		account := make([]byte, 20)
		account[len(account)-1] = byte(i + 1)
		signer, err := accountCodec.BytesToString(account)
		require.NoError(t, err)
		operator, err := validatorCodec.BytesToString(account)
		require.NoError(t, err)
		validator := stakingtypes.Validator{OperatorAddress: operator}
		staking.lastValidators = append(staking.lastValidators, validator)
		identities = append(identities, validatorIdentity{
			account: account, signer: signer, operator: append([]byte(nil), account...), operatorS: operator,
		})
	}

	k := keeper.NewKeeper(storeService, encCfg.Codec, accountCodec, authtypes.NewModuleAddress(types.GovModuleName), staking)
	require.NoError(t, k.Params.Set(ctx, types.DefaultParams()))

	return &fixture{
		ctx: ctx, keeper: k, msgServer: keeper.NewMsgServerImpl(k), query: keeper.NewQueryServerImpl(k),
		staking: staking, accountCodec: accountCodec, validators: identities,
	}
}

func (f *fixture) atHeight(height int64) sdk.Context {
	return f.ctx.WithBlockHeight(height).WithEventManager(sdk.NewEventManager())
}

func submitProof(t testing.TB, f *fixture, height int64, hashByte byte) *types.MsgSubmitProofResponse {
	t.Helper()
	hash := make([]byte, types.ProofHashSize)
	hash[len(hash)-1] = hashByte
	response, err := f.msgServer.SubmitProof(f.atHeight(height), &types.MsgSubmitProof{
		Signer: f.validators[0].signer, ProofHash: hash, ProofType: 7,
	})
	require.NoError(t, err)
	return response
}

func valueLeaf(t testing.TB, saltByte byte, votes []types.ProofVote) types.LeafRevelation {
	t.Helper()
	salt := make([]byte, types.SaltSize)
	salt[len(salt)-1] = saltByte
	return types.LeafRevelation{
		Mode:    types.LeafRevealMode_LEAF_REVEAL_MODE_VALUE,
		Payload: &types.LeafRevelation_Value{Value: &types.RevealedLeaf{Salt: salt, Votes: votes}},
	}
}

func hashLeaf(t testing.TB, leaf types.LeafRevelation) types.LeafRevelation {
	t.Helper()
	hash, err := types.ComputeLeafHash(leaf.GetValue().Salt, leaf.GetValue().Votes)
	require.NoError(t, err)
	return types.LeafRevelation{
		Mode:    types.LeafRevealMode_LEAF_REVEAL_MODE_HASH_ONLY,
		Payload: &types.LeafRevelation_LeafHash{LeafHash: hash[:]},
	}
}

func commitmentFor(t testing.TB, left, right types.LeafRevelation) []byte {
	t.Helper()
	leftValue := left.GetValue()
	rightValue := right.GetValue()
	leftHash, err := types.ComputeLeafHash(leftValue.Salt, leftValue.Votes)
	require.NoError(t, err)
	rightHash, err := types.ComputeLeafHash(rightValue.Salt, rightValue.Votes)
	require.NoError(t, err)
	root := types.ComputeCommitmentRoot(leftHash, rightHash)
	return root[:]
}
