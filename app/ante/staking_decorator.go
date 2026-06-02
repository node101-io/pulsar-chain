package ante

import (
	errorsmod "cosmossdk.io/errors"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	keyregistrytypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

type StakingDecorator struct {
	keyregistryKeeper *keyregistrykeeper.Keeper
}

func NewStakingDecorator(keyregistryKeeper *keyregistrykeeper.Keeper) StakingDecorator {
	return StakingDecorator{
		keyregistryKeeper: keyregistryKeeper,
	}
}

func (d StakingDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	for _, msg := range tx.GetMsgs() {
		createMsg, ok := msg.(*stakingtypes.MsgCreateValidator)
		if !ok {
			continue
		}

		if err := d.handleValidatorCreateTx(ctx, createMsg); err != nil {
			return ctx, err
		}
	}

	return next(ctx, tx, simulate)
}

func (d StakingDecorator) handleValidatorCreateTx(
	ctx sdk.Context,
	msg *stakingtypes.MsgCreateValidator,
) error {
	if msg.Pubkey == nil {
		return stakingtypes.ErrEmptyValidatorPubKey
	}

	pubKey, ok := msg.Pubkey.GetCachedValue().(cryptotypes.PubKey)
	if !ok || pubKey == nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidPubKey, "failed to unpack validator pubkey")
	}

	exists, err := d.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, pubKey.Bytes())
	if err != nil {
		return err
	}

	if !exists {
		return keyregistrytypes.ErrValidatorNotRegistered
	}

	return nil
}
