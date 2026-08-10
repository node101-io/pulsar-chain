package types_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	minafield "github.com/node101-io/mina-signer-go/field"
	merkle "github.com/node101-io/mina-signer-go/merklelist"
	bridgetypes "github.com/node101-io/pulsar-chain/x/bridge/types"
	"github.com/stretchr/testify/require"
)

type rootVector struct {
	Base64  string `json:"base64"`
	Decimal string `json:"decimal"`
}

type actionsRootVectors struct {
	EmptyRoot         rootVector `json:"emptyRoot"`
	SingleDepositRoot struct {
		Action struct {
			BlockHeight int64  `json:"blockHeight"`
			XCoordinate string `json:"xCoordinate"`
			IsOdd       bool   `json:"isOdd"`
			ActionType  string `json:"actionType"`
			Amount      int64  `json:"amount"`
		} `json:"action"`
		rootVector
	} `json:"singleDepositRoot"`
}

func loadActionsRootVectors(t *testing.T) actionsRootVectors {
	t.Helper()

	path := filepath.Join("..", "..", "..", "scripts", "vote-ext-verifier", "actions-root-vectors.json")
	bz, err := os.ReadFile(path)
	require.NoError(t, err)

	var vectors actionsRootVectors
	require.NoError(t, json.Unmarshal(bz, &vectors))
	return vectors
}

func requireRootMatchesVector(t *testing.T, root []byte, vector rootVector) {
	t.Helper()

	expected, err := base64.StdEncoding.DecodeString(vector.Base64)
	require.NoError(t, err)
	require.Equal(t, expected, root)

	element, err := minafield.NewField().FromBytes(root)
	require.NoError(t, err)
	require.Equal(t, vector.Decimal, element.String())
}

// TODO: Verify action-root construction against an authoritative Mina/o1js
// implementation. The JavaScript test currently validates only canonical root decoding.
func TestActionsRootCrossLanguageVectors(t *testing.T) {
	vectors := loadActionsRootVectors(t)
	requireRootMatchesVector(t, bridgetypes.DefaultActionsReducedRoot(), vectors.EmptyRoot)

	actionVector := vectors.SingleDepositRoot.Action
	require.Equal(t, "ACTION_TYPE_DEPOSIT", actionVector.ActionType)

	xCoordinate, err := base64.StdEncoding.DecodeString(actionVector.XCoordinate)
	require.NoError(t, err)

	action := bridgetypes.Action{
		BlockHeight: actionVector.BlockHeight,
		XCoordinate: xCoordinate,
		IsOdd:       actionVector.IsOdd,
		ActionType:  bridgetypes.ActionType_ACTION_TYPE_DEPOSIT,
		Amount:      actionVector.Amount,
	}
	actionField, err := action.ToFieldElement()
	require.NoError(t, err)

	list := merkle.NewMerkleList(bridgetypes.ActionsReducedRootMerkleListPrefixV1)
	require.NoError(t, list.Append(actionField.Bytes()))
	requireRootMatchesVector(t, list.Root(), vectors.SingleDepositRoot.rootVector)
}
