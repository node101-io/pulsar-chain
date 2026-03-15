package keyregistry

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// AutoCLIOptions implements the autocli.HasAutoCLIConfig interface.
func (am AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: types.Query_serviceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "Params",
					Use:       "params",
					Short:     "Shows the parameters of the module",
				},
				{
					RpcMethod:      "GetUserMinaAddress",
					Use:            "get-user-mina-address [user-cosmos-address]",
					Short:          "Query GetUserMinaAddress",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "user_cosmos_address", Varargs: true}},
				},

				{
					RpcMethod:      "GetUserCosmosAddress",
					Use:            "get-user-cosmos-address [user-mina-address]",
					Short:          "Query GetUserCosmosAddress",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "user_mina_address", Varargs: true}},
				},

				{
					RpcMethod:      "GetValidatorMinaAddress",
					Use:            "get-validator-mina-pub-key [validator-cosmos-address]",
					Short:          "Query GetValidatorMinaAddress",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validator_cosmos_address", Varargs: true}},
				},

				{
					RpcMethod:      "GetValidatorCosmosAddress",
					Use:            "get-validator-cosmos-address [validator-mina-address]",
					Short:          "Query GetValidatorCosmosAddress",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validator_mina_address", Varargs: true}},
				},

				// this line is used by ignite scaffolding # autocli/query
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              types.Msg_serviceDesc.ServiceName,
			EnhanceCustomCommand: true, // only required if you want to use the custom command
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "UpdateParams",
					Skip:      true, // skipped because authority gated
				},
				{
					RpcMethod:      "RegisterKeys",
					Use:            "register-keys [cosmos-signature] [mina-signature] [cosmos-public-key] [mina-public-key]",
					Short:          "Send a registerKeys tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "cosmos_signature"}, {ProtoField: "mina_signature"}, {ProtoField: "cosmos_public_key"}, {ProtoField: "mina_public_key", Varargs: true}},
				},
				// this line is used by ignite scaffolding # autocli/tx
			},
		},
	}
}
