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
					RpcMethod:      "GetMinaPubKey",
					Use:            "get-mina-pub-key [cosmos-pub-key]",
					Short:          "Query GetMinaPubKey",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "cosmos_pub_key", Varargs: true}},
				},

				{
					RpcMethod:      "GetCosmosPubKey",
					Use:            "get-cosmos-pub-key [mina-pub-key]",
					Short:          "Query GetCosmosPubKey",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "mina_pub_key", Varargs: true}},
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
