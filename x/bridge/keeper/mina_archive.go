package keeper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/node101-io/mina-signer-go/address"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

var ArchiveGraphQLEndpoint = "https://devnet-archive-node-api.gcp.o1test.net"

type GraphQLResponse struct {
	Data struct {
		NetworkState struct {
			MaxBlockHeight struct {
				PendingMaxBlockHeight int `json:"pendingMaxBlockHeight"`
			} `json:"maxBlockHeight"`
		} `json:"networkState"`
	} `json:"data"`
}

type GraphQLRequest struct {
	Query string `json:"query"`
}

const (
	pulsarActionTypeIndex   = 0
	pulsarActionAmountIndex = 3
	pulsarActionFieldCount  = 7
)

type archiveActionsResponse struct {
	Data archiveActionsData `json:"data"`
}

type archiveActionsData struct {
	Actions []archiveActionGroup `json:"actions"`
	Blocks  []archiveBlockGroup  `json:"blocks"`
}

type archiveActionGroup struct {
	BlockInfo  archiveActionBlockInfo `json:"blockInfo"`
	ActionData []archiveActionData    `json:"actionData"`
}

type archiveActionBlockInfo struct {
	Height int `json:"height"`
}

type archiveActionData struct {
	Data            []string               `json:"data"`
	TransactionInfo archiveTransactionInfo `json:"transactionInfo"`
}

type archiveTransactionInfo struct {
	Hash string `json:"hash"`
}

type archiveBlockGroup struct {
	Transactions archiveBlockTransactions `json:"transactions"`
}

type archiveBlockTransactions struct {
	ZkappCommands []archiveZkappCommand `json:"zkappCommands"`
}

type archiveZkappCommand struct {
	Hash     string `json:"hash"`
	FeePayer string `json:"feePayer"`
}

func GetMinaBlockHeight() (int64, error) {

	const query = `
query GetMinaBlockHeights {
  networkState {
    maxBlockHeight {
      pendingMaxBlockHeight
    }
  }
}`

	body, err := json.Marshal(GraphQLRequest{
		Query: query,
	})
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest("POST", ArchiveGraphQLEndpoint, bytes.NewBuffer(body))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("request failed: %s\n%s", resp.Status, string(respBody))
	}

	fmt.Println(string(respBody))

	var result GraphQLResponse

	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, err
	}

	return int64(result.Data.NetworkState.MaxBlockHeight.PendingMaxBlockHeight), nil
}

func fetchActions(contractAddress string, start, end int64) ([]types.Action, error) {
	blockLimit := int(end - start + 1)
	if blockLimit < 1 {
		blockLimit = 1
	}

	query := `
query ($actionInput: ActionFilterOptionsInput!, $blockQuery: BlockQueryInput!, $blockLimit: Int!) {
  actions(input: $actionInput) {
    blockInfo {
      height
    }
    actionData {
      data
      transactionInfo {
        hash
      }
    }
  }
  blocks(query: $blockQuery, sortBy: BLOCKHEIGHT_ASC, limit: $blockLimit) {
    transactions {
      zkappCommands {
        hash
        feePayer
      }
    }
  }
}`

	body := map[string]any{
		"query": query,
		"variables": map[string]any{
			"actionInput": map[string]any{
				"address": contractAddress,
				"status":  "CANONICAL",
				"from":    start,
				"to":      end,
			},
			"blockQuery": map[string]any{
				"blockHeight_gte": start,
				"blockHeight_lt":  end + 1,
				"canonical":       true,
				"inBestChain":     true,
			},
			"blockLimit": blockLimit,
		},
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(ArchiveGraphQLEndpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %s\n%s", resp.Status, string(out))
	}

	var result archiveActionsResponse

	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}

	feePayerByHash := make(map[string]string)
	for _, block := range result.Data.Blocks {
		for _, zkappCommand := range block.Transactions.ZkappCommands {
			feePayerByHash[zkappCommand.Hash] = zkappCommand.FeePayer
		}
	}

	actions := make([]types.Action, 0)
	for _, block := range result.Data.Actions {
		for _, actionData := range block.ActionData {
			fetchedAction, err := fetchedActionFromActionData(
				block.BlockInfo.Height,
				feePayerByHash[actionData.TransactionInfo.Hash],
				actionData.Data,
			)
			if err != nil {
				return nil, err
			}
			if fetchedAction == nil {
				continue
			}

			actions = append(actions, *fetchedAction)
		}
	}

	return actions, nil
}

func fetchedActionFromActionData(blockHeight int, feePayer string, data []string) (*types.Action, error) {
	actionType, amount, err := contractActionTypeAndAmountFromActionData(data)
	if err != nil {
		return nil, err
	}

	if actionType == types.ActionType_UNSPECIFIED {
		return nil, nil
	}

	minaAddr, err := address.NewAddress(feePayer).Marshal()
	if err != nil {
		return nil, err
	}

	return &types.Action{
		BlockHeight: int64(blockHeight),
		FeePayer:    minaAddr,
		ActionType:  actionType,
		Amount:      amount,
	}, nil
}

func contractActionTypeAndAmountFromActionData(data []string) (types.ActionType, int64, error) {
	if len(data) < pulsarActionFieldCount {
		return 0, 0, fmt.Errorf("invalid action data length: got %d fields, expected at least %d", len(data), pulsarActionFieldCount)
	}

	actionType, err := strconv.Atoi(data[pulsarActionTypeIndex])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid action type %q: %w", data[pulsarActionTypeIndex], err)
	}

	amount, err := strconv.ParseInt(data[pulsarActionAmountIndex], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid action amount %q: %w", data[pulsarActionAmountIndex], err)
	}

	if amount <= 0 {
		return 0, 0, fmt.Errorf("invalid non-positive action amount: %d", amount)
	}

	if actionType == int(types.ActionType_UNSPECIFIED) {
		return types.ActionType_UNSPECIFIED, 0, nil
	}

	return types.ActionType(actionType), amount, nil
}
