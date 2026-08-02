package types

import (
	errorsmod "cosmossdk.io/errors"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/mina-signer-go/publickey"
)

func (act *Action) ToFieldElement() (*field.FieldElement, error) {
	if act == nil {
		return nil, ErrNilAction
	}

	if act.BlockHeight <= 0 {
		return nil, errorsmod.Wrapf(
			ErrInvalidActionBlockHeight,
			"got %d",
			act.BlockHeight,
		)
	}

	if act.Amount <= 0 {
		return nil, errorsmod.Wrapf(
			ErrInvalidActionAmount,
			"got %d",
			act.Amount,
		)
	}

	switch act.ActionType {
	case ActionType_ACTION_TYPE_DEPOSIT, ActionType_ACTION_TYPE_WITHDRAW:
	default:
		return nil, errorsmod.Wrapf(
			ErrInvalidActionType,
			"got %s",
			act.ActionType.String(),
		)
	}

	// networkID is intentionally empty here because mina-signer-go does not use it when parsing raw Mina public-key bytes.
	pk, err := publickey.NewPublicKeyFromBytes(act.FeePayer, "")
	if err != nil {
		return nil, errorsmod.Wrap(
			ErrInvalidActionFeePayer,
			err.Error(),
		)
	}

	x, isOdd, err := pk.ToFields()
	if err != nil {
		return nil, errorsmod.Wrap(
			ErrActionToFieldFailed,
			err.Error(),
		)
	}

	f := field.NewField()
	p := poseidon.NewPoseidon()

	actionField, err := p.HashFieldElementsWithPrefix(
		ActionHashPoseidonPrefixV1,
		f.FromUint64(uint64(act.BlockHeight)),
		x,
		isOdd,
		f.FromUint64(uint64(act.ActionType)),
		f.FromUint64(uint64(act.Amount)),
	)
	if err != nil {
		return nil, errorsmod.Wrap(
			ErrActionToFieldFailed,
			err.Error(),
		)
	}

	return actionField, nil
}
