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

func SimulateMsgUpdateKeys(
	ak types.AuthKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
	txGen client.TxConfig,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		actorType := randomActorType(r)
		msgType := sdk.MsgTypeURL(&types.MsgUpdateUserKeys{})
		if actorType == types.ActorType_VALIDATOR {
			msgType = sdk.MsgTypeURL(&types.MsgUpdateValidatorKeys{})
		}
		genesis, err := k.ExportGenesis(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "unable to export keyregistry genesis"), nil, err
		}

		msg, simAccount, noOpReason, err := buildUpdateKeysMsg(r, ctx, k, actorType, genesis, accs)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "unable to build UpdateKeys msg"), nil, err
		}
		if noOpReason != "" {
			return simtypes.NoOpMsg(types.ModuleName, msgType, noOpReason), nil, nil
		}

		return deliverKeyregistryTx(r, app, txGen, ak, bk, msg, ctx, simAccount)
	}
}
