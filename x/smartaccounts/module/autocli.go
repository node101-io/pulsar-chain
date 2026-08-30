package smartaccounts

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"
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
					RpcMethod: "GetSessionKeysByIdentity",
					Use:       "get-session-keys-by-identity [identity]",
					Short:     "Query GetSessionKeysByIdentity",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "identity"},
					},
				},
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
					RpcMethod: "AddPublicKey",
					Use:       "add-public-key [verification-id] [session-public-key] [expires-at-height] [identity]",
					Short:     "Add a session public key to a smart account",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "verification_id"},
						{ProtoField: "public_key_inputs.session_public_key"},
						{ProtoField: "public_key_inputs.expires_at_height"},
						{ProtoField: "public_key_inputs.identity"},
					},
				},
			},
		},
	}
}
