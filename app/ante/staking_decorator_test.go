package ante

import (
	"bytes"
	"testing"

	"cosmossdk.io/math"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdked25519 "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	keyregistrytypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"
)

// stakingDecoratorTx exposes just enough sdk.Tx behavior for staking decorator tests.
type stakingDecoratorTx struct {
	msgs []sdk.Msg
}

func (tx stakingDecoratorTx) GetMsgs() []sdk.Msg { return tx.msgs }

func (tx stakingDecoratorTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

// newValidatorConsensusPubKeyForTest generates a real ed25519 validator consensus key.
// Using the real key shape keeps keyregistry validation aligned with production behavior.
func newValidatorConsensusPubKeyForTest() cryptotypes.PubKey {
	return sdked25519.GenPrivKey().PubKey()
}

// newCreateValidatorMsgForTest builds a real create-validator message for decorator coverage.
// That lets the tests exercise the same pubkey packing path as the staking module.
func newCreateValidatorMsgForTest(t *testing.T, pubKey cryptotypes.PubKey) *stakingtypes.MsgCreateValidator {
	t.Helper()

	msg, err := stakingtypes.NewMsgCreateValidator(
		sdk.ValAddress(bytes.Repeat([]byte{0x01}, 20)).String(),
		pubKey,
		sdk.NewInt64Coin(sdk.DefaultBondDenom, 1),
		stakingtypes.NewDescription("validator", "", "", "", ""),
		stakingtypes.NewCommissionRates(math.LegacyZeroDec(), math.LegacyOneDec(), math.LegacyZeroDec()),
		math.OneInt(),
	)
	require.NoError(t, err)

	return msg
}

// newDelegateMsgForTest provides a non-create-validator staking message for bypass coverage.
func newDelegateMsgForTest() *stakingtypes.MsgDelegate {
	return stakingtypes.NewMsgDelegate(
		sdk.AccAddress(bytes.Repeat([]byte{0x02}, 20)).String(),
		sdk.ValAddress(bytes.Repeat([]byte{0x03}, 20)).String(),
		sdk.NewInt64Coin(sdk.DefaultBondDenom, 1),
	)
}

// registerValidatorPubKeyForTest stores a minimal validator key mapping directly in genesis state.
// This keeps the test focused on staking decorator behavior rather than registration signatures.
func registerValidatorPubKeyForTest(
	t *testing.T,
	ctx sdk.Context,
	keeper *keyregistrykeeper.Keeper,
	pubKey cryptotypes.PubKey,
) {
	t.Helper()

	minaPrivateKey := newMinaPrivateKeyForTest(t, 31)

	err := keeper.InitGenesis(ctx, keyregistrytypes.GenesisState{
		Params:       keyregistrytypes.DefaultParams(),
		UserKeyPairs: keyregistrytypes.DefaultUserPublicKeyPairs(),
		ValidatorKeyPairs: []*keyregistrytypes.ValidatorPublicKeyPair{
			{
				CosmosKey: pubKey.Bytes(),
				MinaKey:   minaPublicKeyBytesForTest(t, minaPrivateKey),
			},
		},
	})
	require.NoError(t, err)
}

// Non-validator staking messages should bypass validator registration checks entirely.
// This keeps the decorator narrowly focused on create-validator transactions.
func TestStakingDecoratorIgnoresNonCreateValidatorMsgs(t *testing.T) {
	t.Parallel()

	ctx, keeper := newKeyregistryKeeperForTest(t)
	nextCalled := false

	_, err := NewStakingDecorator(keeper).AnteHandle(
		ctx,
		stakingDecoratorTx{msgs: []sdk.Msg{newDelegateMsgForTest()}},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			nextCalled = true
			return nextCtx, nil
		},
	)

	require.NoError(t, err)
	require.True(t, nextCalled)
}

// Create-validator transactions must fail when the consensus key has no keyregistry entry.
// This prevents validators from registering on staking before their Mina mapping exists.
func TestStakingDecoratorRejectsUnregisteredValidatorCreateKey(t *testing.T) {
	t.Parallel()

	ctx, keeper := newKeyregistryKeeperForTest(t)
	nextCalled := false

	_, err := NewStakingDecorator(keeper).AnteHandle(
		ctx,
		stakingDecoratorTx{msgs: []sdk.Msg{newCreateValidatorMsgForTest(t, newValidatorConsensusPubKeyForTest())}},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			nextCalled = true
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, keyregistrytypes.ErrValidatorNotRegistered)
	require.False(t, nextCalled)
}

// A registered validator consensus key should pass straight through to the next ante handler.
// This is the main success path for the decorator.
func TestStakingDecoratorAllowsRegisteredValidatorCreateKey(t *testing.T) {
	t.Parallel()

	ctx, keeper := newKeyregistryKeeperForTest(t)
	pubKey := newValidatorConsensusPubKeyForTest()
	registerValidatorPubKeyForTest(t, ctx, keeper, pubKey)
	nextCalled := false

	_, err := NewStakingDecorator(keeper).AnteHandle(
		ctx,
		stakingDecoratorTx{msgs: []sdk.Msg{newCreateValidatorMsgForTest(t, pubKey)}},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			nextCalled = true
			return nextCtx, nil
		},
	)

	require.NoError(t, err)
	require.True(t, nextCalled)
}

// A single later create-validator message must still be checked even if earlier msgs are harmless.
// This confirms the decorator scans the entire transaction rather than only the first staking msg.
func TestStakingDecoratorScansAllMessagesInTx(t *testing.T) {
	t.Parallel()

	ctx, keeper := newKeyregistryKeeperForTest(t)

	_, err := NewStakingDecorator(keeper).AnteHandle(
		ctx,
		stakingDecoratorTx{msgs: []sdk.Msg{
			newDelegateMsgForTest(),
			newCreateValidatorMsgForTest(t, newValidatorConsensusPubKeyForTest()),
		}},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			t.Fatal("next handler should not be called when a later create-validator msg is invalid")
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, keyregistrytypes.ErrValidatorNotRegistered)
}

// Create-validator messages without a pubkey should fail before any keyregistry lookup happens.
// This locks down the explicit empty-pubkey branch in handleValidatorCreateTx.
func TestStakingDecoratorRejectsEmptyValidatorPubKey(t *testing.T) {
	t.Parallel()

	ctx, keeper := newKeyregistryKeeperForTest(t)
	msg := newCreateValidatorMsgForTest(t, newValidatorConsensusPubKeyForTest())
	msg.Pubkey = nil
	nextCalled := false

	_, err := NewStakingDecorator(keeper).AnteHandle(
		ctx,
		stakingDecoratorTx{msgs: []sdk.Msg{msg}},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			nextCalled = true
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, stakingtypes.ErrEmptyValidatorPubKey)
	require.False(t, nextCalled)
}

// Malformed packed pubkeys should fail the decorator before keyregistry state is consulted.
// A wrong Any payload is the smallest stable way to cover the unpack error branch.
func TestStakingDecoratorRejectsInvalidPackedValidatorPubKey(t *testing.T) {
	t.Parallel()

	ctx, keeper := newKeyregistryKeeperForTest(t)
	msg := newCreateValidatorMsgForTest(t, newValidatorConsensusPubKeyForTest())
	msg.Pubkey = &codectypes.Any{TypeUrl: "/example.NotAPubKey"}
	nextCalled := false

	_, err := NewStakingDecorator(keeper).AnteHandle(
		ctx,
		stakingDecoratorTx{msgs: []sdk.Msg{msg}},
		false,
		func(nextCtx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
			nextCalled = true
			return nextCtx, nil
		},
	)

	require.ErrorIs(t, err, sdkerrors.ErrInvalidPubKey)
	require.ErrorContains(t, err, "failed to unpack validator pubkey")
	require.False(t, nextCalled)
}
