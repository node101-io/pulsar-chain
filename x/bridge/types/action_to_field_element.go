package types

import (
	errorsmod "cosmossdk.io/errors"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
)

func (act *Action) ToFieldElement(isValidAction bool) (*field.FieldElement, error) {
	if act == nil {
		return nil, ErrNilAction
	}

	f := field.NewField()
	p := poseidon.NewPoseidon()

	xCoordinateToField, err := f.FromBytes(act.XCoordinate)
	if err != nil {
		return nil, errorsmod.Wrap(
			ErrInvalidActionXCoordinate,
			err.Error(),
		)
	}

	actionField, err := p.HashFieldElementsWithPrefix(
		ActionHashPoseidonPrefixV1,
		boolToField(isValidAction),
		xCoordinateToField,
		boolToField(act.IsOdd),
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

func boolToField(value bool) *field.FieldElement {
	f := field.NewField()

	if value {
		return f.One()
	}

	return f.Zero()
}
