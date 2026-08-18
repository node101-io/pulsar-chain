package verification

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func (AppModule) AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: types.Query_serviceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{RpcMethod: "Params", Use: "params", Short: "Show verification parameters"},
				{RpcMethod: "Proof", Use: "proof [submission-height] [index-in-block]", Short: "Show a proof by key", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "submission_height"}, {ProtoField: "index_in_block"}}},
				{RpcMethod: "ProofByHash", Use: "proof-by-hash [proof-hash]", Short: "Show a proof by hash", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "proof_hash"}}},
				{RpcMethod: "ProofsByHeight", Use: "proofs-by-height [submission-height]", Short: "List proofs submitted at a height", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "submission_height"}}},
				{RpcMethod: "Commitment", Use: "commitment [validator] [commitment-height]", Short: "Show a validator commitment", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validator"}, {ProtoField: "commitment_height"}}},
				{RpcMethod: "CommitmentsByValidator", Use: "commitments-by-validator [validator]", Short: "List retained validator commitments", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "validator"}}},
				{RpcMethod: "Vote", Use: "vote [submission-height] [index-in-block] [validator]", Short: "Show a validator vote", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "submission_height"}, {ProtoField: "index_in_block"}, {ProtoField: "validator"}}},
				{RpcMethod: "VerificationVotes", Use: "verification-votes [submission-height] [index-in-block]", Short: "List votes for a proof", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "submission_height"}, {ProtoField: "index_in_block"}}},
				{RpcMethod: "ProofTally", Use: "proof-tally [submission-height] [index-in-block]", Short: "Show a proof tally", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "submission_height"}, {ProtoField: "index_in_block"}}},
				{RpcMethod: "FinalProofResult", Use: "final-proof-result [submission-height] [index-in-block]", Short: "Show a finalized proof result", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "submission_height"}, {ProtoField: "index_in_block"}}},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service:              types.Msg_serviceDesc.ServiceName,
			EnhanceCustomCommand: true,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{RpcMethod: "SubmitProof", Use: "submit-proof [proof-hash] [proof-type]", Short: "Submit a proof hash", PositionalArgs: []*autocliv1.PositionalArgDescriptor{{ProtoField: "proof_hash"}, {ProtoField: "proof_type"}}},
				{RpcMethod: "UpdateParams", Skip: true},
			},
		},
	}
}
