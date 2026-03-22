package vote_ext

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"cosmossdk.io/store/types"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtestutil "github.com/cosmos/cosmos-sdk/x/staking/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/keys"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	keyregistrytypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
	voteextkeeper "github.com/node101-io/pulsar-chain/x/voteexthandler/keeper"
	voteexthandlertypes "github.com/node101-io/pulsar-chain/x/voteexthandler/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type voteExtTestFixture struct {
	ctx           sdk.Context
	handler       *VoteExtHandler
	validatorAddr sdk.ConsAddress
}

func newVoteExtTestFixture(t *testing.T) voteExtTestFixture {
	t.Helper()

	encCfg := moduletestutil.MakeTestEncodingConfig()
	accountCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	validatorCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix())
	consensusCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32ConsensusAddrPrefix())

	stakingStoreKey := types.NewKVStoreKey(stakingtypes.StoreKey)
	keyregistryStoreKey := types.NewKVStoreKey(keyregistrytypes.StoreKey)
	voteExtStoreKey := types.NewKVStoreKey(voteexthandlertypes.StoreKey)

	ctx := testutil.DefaultContextWithKeys(
		map[string]*types.KVStoreKey{
			stakingtypes.StoreKey:        stakingStoreKey,
			keyregistrytypes.StoreKey:    keyregistryStoreKey,
			voteexthandlertypes.StoreKey: voteExtStoreKey,
		},
		map[string]*types.TransientStoreKey{},
		map[string]*types.MemoryStoreKey{},
	)

	ctrl := gomock.NewController(t)
	accountKeeper := stakingtestutil.NewMockAccountKeeper(ctrl)
	accountKeeper.EXPECT().GetModuleAddress(stakingtypes.BondedPoolName).Return(
		authtypes.NewEmptyModuleAccount(stakingtypes.BondedPoolName).GetAddress(),
	)
	accountKeeper.EXPECT().GetModuleAddress(stakingtypes.NotBondedPoolName).Return(
		authtypes.NewEmptyModuleAccount(stakingtypes.NotBondedPoolName).GetAddress(),
	)
	accountKeeper.EXPECT().AddressCodec().Return(accountCodec).AnyTimes()

	bankKeeper := stakingtestutil.NewMockBankKeeper(ctrl)
	authorityBytes := authtypes.NewModuleAddress(govtypes.ModuleName)
	authority, err := accountCodec.BytesToString(authorityBytes)
	require.NoError(t, err)

	stakingKeeper := stakingkeeper.NewKeeper(
		encCfg.Codec,
		runtime.NewKVStoreService(stakingStoreKey),
		accountKeeper,
		bankKeeper,
		authority,
		validatorCodec,
		consensusCodec,
	)
	require.NoError(t, stakingKeeper.SetParams(ctx, stakingtypes.DefaultParams()))

	keyregistryKeeper := keyregistrykeeper.NewKeeper(
		runtime.NewKVStoreService(keyregistryStoreKey),
		encCfg.Codec,
		accountCodec,
		authorityBytes,
	)
	require.NoError(t, keyregistryKeeper.Params.Set(ctx, keyregistrytypes.DefaultParams()))

	voteextKeeper := voteextkeeper.NewKeeper(
		runtime.NewKVStoreService(voteExtStoreKey),
		encCfg.Codec,
		accountCodec,
		authorityBytes,
	)
	require.NoError(t, voteextKeeper.Params.Set(ctx, voteexthandlertypes.DefaultParams()))

	minaSeed := sha256.Sum256([]byte("vote-extension-success-test"))
	minaPrivateKey := keys.NewPrivateKeyFromBytes(minaSeed)
	minaPublicKey := minaPrivateKey.ToPublicKey()
	handler := NewVoteExtHandler(
		keyregistryKeeper,
		voteextKeeper,
		&voteexthandlertypes.SecondaryKey{
			SecretKey: &minaPrivateKey,
			PublicKey: &minaPublicKey,
		},
		*stakingKeeper,
	)

	consensusPubKey := ed25519.GenPrivKey().PubKey()
	validator := stakingtestutil.NewValidator(t, sdk.ValAddress(consensusPubKey.Address()), consensusPubKey)
	require.NoError(t, stakingKeeper.SetValidator(ctx, validator))
	require.NoError(t, stakingKeeper.SetValidatorByConsAddr(ctx, validator))

	return voteExtTestFixture{
		ctx:           ctx,
		handler:       handler,
		validatorAddr: sdk.ConsAddress(consensusPubKey.Address()),
	}
}

func TestVoteExtensionLifecycleSuccess(t *testing.T) {
	fixture := newVoteExtTestFixture(t)

	root1 := bytes.Repeat([]byte{0x11}, 32)
	root2 := bytes.Repeat([]byte{0x22}, 32)
	root3 := bytes.Repeat([]byte{0x33}, 32)

	processProposal := fixture.handler.ProcessProposalHandler()

	processResp, err := processProposal(
		fixture.ctx.WithBlockHeader(cmtproto.Header{Height: 1, AppHash: root1}),
		&abci.RequestProcessProposal{Height: 2},
	)
	require.NoError(t, err)
	require.Equal(t, abci.ResponseProcessProposal_ACCEPT, processResp.Status)

	processResp, err = processProposal(
		fixture.ctx.WithBlockHeader(cmtproto.Header{Height: 2, AppHash: root2}),
		&abci.RequestProcessProposal{Height: 3},
	)
	require.NoError(t, err)
	require.Equal(t, abci.ResponseProcessProposal_ACCEPT, processResp.Status)

	extendResp, err := fixture.handler.ExtendVoteHandler()(fixture.ctx, &abci.RequestExtendVote{Height: 3})
	require.NoError(t, err)
	require.NotEmpty(t, extendResp.VoteExtension)

	prepareResp, err := fixture.handler.PrepareProposalHandler()(fixture.ctx, &abci.RequestPrepareProposal{Height: 4})
	require.NoError(t, err)
	require.Len(t, prepareResp.Txs, 1)
	require.True(t, bytes.HasPrefix(prepareResp.Txs[0], voteexthandlertypes.VoteExtMarker))

	processResp, err = processProposal(
		fixture.ctx.WithBlockHeader(cmtproto.Header{Height: 3, AppHash: root3}),
		&abci.RequestProcessProposal{
			Height: 4,
			Txs:    prepareResp.Txs,
		},
	)
	require.NoError(t, err)
	require.Equal(t, abci.ResponseProcessProposal_ACCEPT, processResp.Status)

	verifyResp, err := fixture.handler.VerifyVoteExtensionHandler()(
		fixture.ctx,
		&abci.RequestVerifyVoteExtension{
			Height:           3,
			ValidatorAddress: fixture.validatorAddr,
			VoteExtension:    extendResp.VoteExtension,
		},
	)
	require.NoError(t, err)
	require.Equal(t, abci.ResponseVerifyVoteExtension_ACCEPT, verifyResp.Status)
}
