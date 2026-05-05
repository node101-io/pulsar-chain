package keeper_test

import (
	"math/big"
	"testing"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	keyregistrytypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/node101-io/pulsar-chain/x/votepersistence/keeper"
	"github.com/node101-io/pulsar-chain/x/votepersistence/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var queryMockCosmosSignature = []byte("cosmosSig")
var queryMockMinaSignature = []byte("minaSig")

func newBondedValidator(t *testing.T, power int64) stakingtypes.Validator {
	t.Helper()

	consPubKey := ed25519.GenPrivKey().PubKey()
	validator, err := stakingtypes.NewValidator(
		sdk.ValAddress(consPubKey.Address()).String(),
		consPubKey,
		stakingtypes.Description{},
	)
	require.NoError(t, err)

	validator = validator.UpdateStatus(stakingtypes.Bonded)
	validator.Tokens = sdk.TokensFromConsensusPower(power, sdk.DefaultPowerReduction)

	return validator
}

func generateMinaPublicKey(t *testing.T, seed [32]byte) []byte {
	t.Helper()

	minaPrivKey := keys.NewPrivateKeyFromBytes(seed)
	minaPubKey, err := minaPrivKey.ToPublicKey().Marshal()
	require.NoError(t, err)

	return minaPubKey
}

func registerValidatorKeys(t *testing.T, f *fixture, cosmosPubKey cryptotypes.PubKey, minaPubKey []byte) {
	t.Helper()

	ms := keyregistrykeeper.NewMsgServerImpl(f.keyregistryKeeper)
	creatorAddr := sdk.AccAddress(cosmosPubKey.Address())

	_, err := ms.RegisterKeys(f.ctx, &keyregistrytypes.MsgRegisterKeys{
		Creator:         creatorAddr.String(),
		CosmosSignature: queryMockCosmosSignature,
		MinaSignature:   queryMockMinaSignature,
		CosmosPublicKey: cosmosPubKey.Bytes(),
		MinaPublicKey:   minaPubKey,
		ActorType:       keyregistrytypes.ActorType_VALIDATOR,
	})
	require.NoError(t, err)
}

func calculateExpectedValidatorSetRoot(t *testing.T, validatorSet []stakingtypes.Validator, cosmosToMina map[string][]byte) []byte {
	t.Helper()

	poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)
	merkleRoot := poseidonHash.Hash([]*big.Int{big.NewInt(0)})

	for _, validator := range validatorSet {
		cosmosValidatorPubKey, err := validator.ConsPubKey()
		require.NoError(t, err)

		minaPubKeyBz, ok := cosmosToMina[string(cosmosValidatorPubKey.Bytes())]
		require.True(t, ok)

		var minaPubKey keys.PublicKey
		require.NoError(t, minaPubKey.Unmarshal(minaPubKeyBz))

		input := []*big.Int{minaPubKey.X}
		if minaPubKey.IsOdd {
			input = append(input, big.NewInt(1))
		} else {
			input = append(input, big.NewInt(0))
		}
		input = append(input, big.NewInt(validator.ConsensusPower(sdk.DefaultPowerReduction)))

		hashOfValidator := poseidonHash.Hash(input)
		merkleRoot = poseidonHash.Hash([]*big.Int{merkleRoot, hashOfValidator})
	}

	return merkleRoot.Bytes()
}

// TestVoteExtBodyByHeightInvalidArgumentFail verifies that VoteExtBodyByHeight
// returns an InvalidArgument error when called with a nil request.
func TestVoteExtBodyByHeightInvalidArgumentFail(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.VoteExtBodyByHeight(f.ctx, nil)
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

// TestVoteExtBodyByHeightEarlyBlockInvalidArgumentFail verifies that
// VoteExtBodyByHeight rejects requests for block heights smaller than 4.
func TestVoteExtBodyByHeightEarlyBlockInvalidArgumentFail(t *testing.T) {
	f := initFixture(t)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.VoteExtBodyByHeight(f.ctx, &types.QueryVoteExtBodyByHeightRequest{
		BlockHeight: 3,
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

// TestVoteExtBodyByHeightRequestedBlockNotAvailable verifies that
// VoteExtBodyByHeight rejects requests for the current or future block.
func TestVoteExtBodyByHeightRequestedBlockNotAvailable(t *testing.T) {
	f := initFixture(t)
	ctx := f.ctx.WithBlockHeight(10)

	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.VoteExtBodyByHeight(ctx, &types.QueryVoteExtBodyByHeightRequest{
		BlockHeight: 10,
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

// TestVoteExtBodyByHeightSuccess verifies that VoteExtBodyByHeight constructs
// the expected body using real staking historical info and registered validator
// Mina keys.
func TestVoteExtBodyByHeightSuccess(t *testing.T) {
	f := initFixture(t)

	validatorOne := newBondedValidator(t, 25)
	validatorTwo := newBondedValidator(t, 10)

	consPubKeyOne, err := validatorOne.ConsPubKey()
	require.NoError(t, err)
	consPubKeyTwo, err := validatorTwo.ConsPubKey()
	require.NoError(t, err)

	minaPubKeyOne := generateMinaPublicKey(t, [32]byte{
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
	})
	minaPubKeyTwo := generateMinaPublicKey(t, [32]byte{
		2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2,
	})

	registerValidatorKeys(t, f, consPubKeyOne, minaPubKeyOne)
	registerValidatorKeys(t, f, consPubKeyTwo, minaPubKeyTwo)

	require.NoError(t, f.stakingKeeper.SetHistoricalInfo(f.ctx, 6, &stakingtypes.HistoricalInfo{
		Header: tmproto.Header{AppHash: []byte("current-state-root")},
	}))
	require.NoError(t, f.stakingKeeper.SetHistoricalInfo(f.ctx, 7, &stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{validatorOne, validatorTwo},
	}))

	cosmosToMina := map[string][]byte{
		string(consPubKeyOne.Bytes()): minaPubKeyOne,
		string(consPubKeyTwo.Bytes()): minaPubKeyTwo,
	}

	ctx := f.ctx.WithBlockHeight(10)
	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	queryResp, err := qs.VoteExtBodyByHeight(ctx, &types.QueryVoteExtBodyByHeightRequest{
		BlockHeight: 8,
	})
	require.NoError(t, err)
	require.NotNil(t, queryResp)

	require.Equal(t, &types.VoteExtBody{
		NextValidatorSetHash: calculateExpectedValidatorSetRoot(t, []stakingtypes.Validator{validatorOne, validatorTwo}, cosmosToMina),
		CurrentStateRoot:     []byte("current-state-root"),
		CurrentBlockHeight:   7,
		ActionsReducedRoot:   keeper.ActionsReducedRoot,
	}, queryResp)
}

// TestVoteExtBodyByHeightValidatorMinaKeyNotFound verifies that
// VoteExtBodyByHeight returns a NotFound error when a validator is missing a
// registered Mina key.
func TestVoteExtBodyByHeightValidatorMinaKeyNotFound(t *testing.T) {
	f := initFixture(t)

	validator := newBondedValidator(t, 15)

	require.NoError(t, f.stakingKeeper.SetHistoricalInfo(f.ctx, 6, &stakingtypes.HistoricalInfo{
		Header: tmproto.Header{AppHash: []byte("current-state-root")},
	}))
	require.NoError(t, f.stakingKeeper.SetHistoricalInfo(f.ctx, 7, &stakingtypes.HistoricalInfo{
		Valset: []stakingtypes.Validator{validator},
	}))

	ctx := f.ctx.WithBlockHeight(10)
	qs := keeper.NewQueryServerImpl(f.keeper)
	params := types.DefaultParams()
	require.NoError(t, f.keeper.Params.Set(f.ctx, params))

	_, err := qs.VoteExtBodyByHeight(ctx, &types.QueryVoteExtBodyByHeightRequest{
		BlockHeight: 8,
	})
	require.Error(t, err)

	st, _ := status.FromError(err)
	require.Equal(t, codes.NotFound, st.Code())
}
