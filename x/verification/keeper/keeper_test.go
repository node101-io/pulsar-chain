package keeper_test

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/core/address"
	storetypes "cosmossdk.io/store/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
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
	historicalInfo map[int64]stakingtypes.HistoricalInfo
	historicalErr  map[int64]error
}

func (s *stakingKeeper) GetHistoricalInfo(_ context.Context, height int64) (stakingtypes.HistoricalInfo, error) {
	if err := s.historicalErr[height]; err != nil {
		return stakingtypes.HistoricalInfo{}, err
	}
	info, ok := s.historicalInfo[height]
	if !ok {
		return stakingtypes.HistoricalInfo{}, fmt.Errorf("historical info %d not found", height)
	}
	return info, nil
}

func (s *stakingKeeper) ValidatorAddressCodec() address.Codec {
	return s.validatorCodec
}

type validatorIdentity struct {
	account   []byte
	signer    string
	operator  []byte
	operatorS string
	power     int64
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
	powers := make([]int64, validatorCount)
	for i := range powers {
		powers[i] = 1
	}
	if validatorCount == 1 {
		powers[0] = 100
	}
	if validatorCount == 3 {
		powers = []int64{60, 25, 15}
	}
	return initFixtureWithPowers(t, powers)
}

func initFixtureWithPowers(t testing.TB, powers []int64) *fixture {
	t.Helper()
	encCfg := moduletestutil.MakeTestEncodingConfig(verificationmodule.AppModule{})
	accountCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	validatorCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	storeService := runtime.NewKVStoreService(storeKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_verification")).Ctx

	staking := &stakingKeeper{
		validatorCodec: validatorCodec,
		historicalInfo: make(map[int64]stakingtypes.HistoricalInfo),
		historicalErr:  make(map[int64]error),
	}
	identities := make([]validatorIdentity, 0, len(powers))
	validators := make([]stakingtypes.Validator, 0, len(powers))
	for i, power := range powers {
		account := make([]byte, 20)
		account[len(account)-1] = byte(i + 1)
		signer, err := accountCodec.BytesToString(account)
		require.NoError(t, err)
		operator, err := validatorCodec.BytesToString(account)
		require.NoError(t, err)
		validator := stakingtypes.Validator{
			OperatorAddress: operator,
			Tokens:          sdk.TokensFromConsensusPower(power, sdk.DefaultPowerReduction),
			Status:          stakingtypes.Bonded,
		}
		validators = append(validators, validator)
		identities = append(identities, validatorIdentity{
			account: account, signer: signer, operator: append([]byte(nil), account...), operatorS: operator, power: power,
		})
	}
	for _, height := range []int64{499, 500, 501} {
		staking.historicalInfo[height] = stakingtypes.HistoricalInfo{
			Header: tmproto.Header{Height: height},
			Valset: append([]stakingtypes.Validator(nil), validators...),
		}
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
	msg := proofSubmission(f, hashByte)
	response, err := f.msgServer.SubmitProof(f.atHeight(height), msg)
	require.NoError(t, err)
	return response
}

func proofSubmission(f *fixture, hashByte byte) *types.MsgSubmitProof {
	hash := make([]byte, types.ProofHashSize)
	hash[len(hash)-1] = hashByte
	publicInputsHash := bytesOf(types.PublicInputsHashSize, hashByte+1)
	verificationKeyHash := bytesOf(types.VerificationKeyHashSize, hashByte+2)
	signer := ""
	if len(f.validators) > 0 {
		signer = f.validators[0].signer
	}
	return &types.MsgSubmitProof{
		Signer:              signer,
		ProofHash:           hash,
		ProofType:           types.ProofType_PROOF_TYPE_MINA_PICKLES,
		PublicInputsHash:    publicInputsHash,
		VerificationKeyHash: verificationKeyHash,
	}
}

func bytesOf(size int, value byte) []byte {
	out := make([]byte, size)
	out[len(out)-1] = value
	return out
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
