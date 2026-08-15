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

				{
					RpcMethod:      "GetValidatorSetWithMinaKeys",
					Use:            "get-validator-set-with-mina-keys",
					Short:          "Query a validator set with registered Mina public keys",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{},
				},
				{
					RpcMethod: "GetKeySigningChallenge",
					Use:       "get-key-signing-challenge [operation] [actor-type] [cosmos-public-key] [new-mina-public-key]",
					Short:     "Build the field challenge used for key registration or update",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "operation"},
						{ProtoField: "actor_type"},
						{ProtoField: "cosmos_public_key"},
						{ProtoField: "new_mina_public_key"},
					},
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
					RpcMethod: "RegisterUserKeys",
					Use:       "register-user-keys [cosmos-public-key] [mina-public-key] [mina-signature]",
					Short:     "Register user Cosmos and Mina keys",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "cosmos_public_key"},
						{ProtoField: "mina_public_key"},
						{ProtoField: "mina_signature"},
					},
				},
				{
					RpcMethod: "UpdateUserKeys",
					Use:       "update-user-keys [cosmos-public-key] [new-mina-public-key] [new-key-version] [new-mina-signature]",
					Short:     "Update a user's Mina key",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "cosmos_public_key"},
						{ProtoField: "new_mina_public_key"},
						{ProtoField: "new_key_version"},
						{ProtoField: "new_mina_signature"},
					},
				},
				{
					RpcMethod: "RegisterValidatorKeys",
					Use:       "register-validator-keys [validator-consensus-public-key] [mina-public-key] [mina-signature] [validator-consensus-signature]",
					Short:     "Register validator consensus and Mina keys",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "validator_consensus_public_key"},
						{ProtoField: "mina_public_key"},
						{ProtoField: "mina_signature"},
						{ProtoField: "validator_consensus_signature"},
					},
				},
				{
					RpcMethod: "UpdateValidatorKeys",
					Use:       "update-validator-keys [validator-consensus-public-key] [new-mina-public-key] [new-key-version] [new-mina-signature] [validator-consensus-signature]",
					Short:     "Update a validator's Mina key",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "validator_consensus_public_key"},
						{ProtoField: "new_mina_public_key"},
						{ProtoField: "new_key_version"},
						{ProtoField: "new_mina_signature"},
						{ProtoField: "validator_consensus_signature"},
					},
				},
			},
		},
	}
}
