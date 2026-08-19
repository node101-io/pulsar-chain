package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterLegacyAminoCodec registers concrete oneof variants used by JSON and
// legacy Amino consumers.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&LeafRevelation_LeafHash{}, "pulsarchain/verification/LeafHash", nil)
	cdc.RegisterConcrete(&LeafRevelation_Value{}, "pulsarchain/verification/LeafValue", nil)
	cdc.RegisterConcrete(&QueryProofResponse_Pending{}, "pulsarchain/verification/Pending", nil)
	cdc.RegisterConcrete(&QueryProofResponse_FinalResult{}, "pulsarchain/verification/Final", nil)
}

// RegisterInterfaces exposes the public proof-submission and governance
// messages to the Cosmos SDK interface registry.
func RegisterInterfaces(registrar codectypes.InterfaceRegistry) {
	registrar.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgSubmitProof{},
		&MsgUpdateParams{},
	)
	msgservice.RegisterMsgServiceDesc(registrar, &_Msg_serviceDesc)
}
