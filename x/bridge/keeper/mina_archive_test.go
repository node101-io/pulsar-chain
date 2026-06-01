package keeper

import (
	"fmt"
	"testing"

	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

func TestGetMinaBlockHeights(t *testing.T) {
	heights, err := GetMinaBlockHeight()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("pending:", heights)
}
func TestFetchActions(t *testing.T) {

	heights := 520114

	actions, err := fetchActions(types.ContractAddress, int64(heights-2000), int64(heights))
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%+v\n", actions)

	for _, act := range actions {

		fmt.Println(act.ActionType, act.Amount, act.BlockHeight, act.FeePayer)

	}

}
