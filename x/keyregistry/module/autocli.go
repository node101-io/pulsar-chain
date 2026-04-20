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
					RpcMethod:      "GetUserMinaPublicKey",
					Use:            "get-user-mina-public-key [user-cosmos-public-key]",
					Short:          "Query GetUserMinaPublicKey",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "user_cosmos_public_key", Varargs: true}},
				},

				{
					RpcMethod:      "GetUserCosmosPublicKey",
					Use:            "get-user-cosmos-public-key [user-mina-public-key]",
					Short:          "Query GetUserCosmosPublicKey",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "user_mina_public_key", Varargs: true}},
				},

				{
					RpcMethod:      "GetValidatorMinaPubKey",
					Use:            "get-validator-mina-pub-key [validator-cosmos-address]",
					Short:          "Query GetValidatorMinaAddress",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validator_cosmos_pub_key", Varargs: true}},
				},

				{
					RpcMethod:      "GetValidatorCosmosPubKey",
					Use:            "get-validator-cosmos-address [validator-mina-pub-key]",
					Short:          "Query GetValidatorCosmosAddress",
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
				{
					RpcMethod:      "UpdateKeys",
					Use:            "update-keys ",
					Short:          "Send a UpdateKeys tx",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{},
				},
				// this line is used by ignite scaffolding # autocli/tx
			},
		},
	}
}
