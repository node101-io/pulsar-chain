package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func SimulateMsgRegisterKeys(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
	txGen client.TxConfig,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		msgType := sdk.MsgTypeURL(&types.MsgRegisterKeys{})
		simAccount, noOpReason := randomSimulationAccount(r, accs)
		if noOpReason != "" {
			return simtypes.NoOpMsg(types.ModuleName, msgType, noOpReason), nil, nil
		}

		msg, noOpReason, err := buildRegisterKeysMsg(r, ctx, k, randomActorType(r), simAccount)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "unable to build RegisterKeys msg"), nil, err
		}
		if noOpReason != "" {
			return simtypes.NoOpMsg(types.ModuleName, msgType, noOpReason), nil, nil
		}

		return deliverKeyregistryTx(r, app, txGen, ak, bk, msg, ctx, simAccount)
	}
}
