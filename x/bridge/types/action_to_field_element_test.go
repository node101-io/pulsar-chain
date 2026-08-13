package types_test

import (
	"testing"

	minaaddress "github.com/node101-io/mina-signer-go/address"
	minafield "github.com/node101-io/mina-signer-go/field"
	minapublickey "github.com/node101-io/mina-signer-go/publickey"
	bridgetypes "github.com/node101-io/pulsar-chain/x/bridge/types"
	"github.com/stretchr/testify/require"
)

const testMinaAddress = "B62qjRDirGFRf5dvNcGzMs5oWzQ2VyNcygnoKM2MkxB9PFUp7Utdraf"

func minaActionCoordinates(t *testing.T) ([]byte, bool) {
	t.Helper()

	minaPubKey, err := minaaddress.NewAddress(testMinaAddress).Marshal()
	require.NoError(t, err)

	pubKey, err := minapublickey.NewPublicKeyFromBytes(minaPubKey, "")
	require.NoError(t, err)

	xCoordinate, isOddField, err := pubKey.ToFields()
	require.NoError(t, err)

	return xCoordinate.Bytes(), !isOddField.IsZero()
}

func invalidCurveXCoordinate(t *testing.T) []byte {
	t.Helper()

	f := minafield.NewField()
	for value := uint64(0); value < 1024; value++ {
		xCoordinate := f.FromUint64(value)
		if _, err := minapublickey.NewPublicKeyFromFieldElement(xCoordinate, false, ""); err != nil {
			return xCoordinate.Bytes()
		}
	}

	t.Fatal("could not find a field element outside the Pallas curve")
	return nil
}

func TestActionToFieldElementIncludesPublicKeyParity(t *testing.T) {
	xCoordinate, isOdd := minaActionCoordinates(t)

	action := bridgetypes.Action{
		BlockHeight: 42,
		XCoordinate: xCoordinate,
		IsOdd:       isOdd,
		ActionType:  bridgetypes.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      7,
	}
	oppositeParityAction := action
	oppositeParityAction.IsOdd = !action.IsOdd

	actionField, err := action.ToFieldElement(true)
	require.NoError(t, err)
	oppositeParityField, err := oppositeParityAction.ToFieldElement(true)
	require.NoError(t, err)
	require.False(t, actionField.Equal(oppositeParityField))
}

func TestActionToFieldElementIncludesValidity(t *testing.T) {
	xCoordinate, isOdd := minaActionCoordinates(t)
	action := bridgetypes.Action{
		BlockHeight: 42,
		XCoordinate: xCoordinate,
		IsOdd:       isOdd,
		ActionType:  bridgetypes.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      7,
	}

	validField, err := action.ToFieldElement(true)
	require.NoError(t, err)
	invalidField, err := action.ToFieldElement(false)
	require.NoError(t, err)
	require.False(t, validField.Equal(invalidField))
}

func TestActionToFieldElementDoesNotIncludeBlockHeight(t *testing.T) {
	xCoordinate, isOdd := minaActionCoordinates(t)
	action := bridgetypes.Action{
		BlockHeight: 42,
		XCoordinate: xCoordinate,
		IsOdd:       isOdd,
		ActionType:  bridgetypes.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      7,
	}
	actionAtAnotherHeight := action
	actionAtAnotherHeight.BlockHeight = 99

	actionField, err := action.ToFieldElement(true)
	require.NoError(t, err)
	actionAtAnotherHeightField, err := actionAtAnotherHeight.ToFieldElement(true)
	require.NoError(t, err)
	require.True(t, actionField.Equal(actionAtAnotherHeightField))
}

func TestActionToFieldElementAcceptsFieldCoordinateOutsideCurve(t *testing.T) {
	action := bridgetypes.Action{
		BlockHeight: 42,
		XCoordinate: invalidCurveXCoordinate(t),
		ActionType:  bridgetypes.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      7,
	}

	_, err := action.ToFieldElement(false)
	require.NoError(t, err)
}
