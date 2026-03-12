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
					RpcMethod:      "GetUserMinaPubKey",
					Use:            "get-user-mina-pub-key [user-cosmos-pub-key]",
					Short:          "Query GetUserMinaPubKey",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "user_cosmos_pub_key", Varargs: true}},
				},

				{
					RpcMethod:      "GetUserCosmosPubKey",
					Use:            "get-user-cosmos-pub-key [user-mina-pub-key]",
					Short:          "Query GetUserCosmosPubKey",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "user_mina_pub_key", Varargs: true}},
				},

				{
					RpcMethod:      "GetValidatorMinaPubKey",
					Use:            "get-validator-mina-pub-key [validator-cosmos-pub-key]",
					Short:          "Query GetValidatorMinaPubKey",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validator_cosmos_pub_key", Varargs: true}},
				},

				{
					RpcMethod:      "GetValidatorCosmosPubKey",
					Use:            "get-validator-cosmos-pub-key [validator-mina-pub-key]",
					Short:          "Query GetValidatorCosmosPubKey",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validator_mina_pub_key", Varargs: true}},
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
