package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&LeafRevelation_LeafHash{}, "pulsarchain/verification/LeafHash", nil)
	cdc.RegisterConcrete(&LeafRevelation_Value{}, "pulsarchain/verification/LeafValue", nil)
	cdc.RegisterConcrete(&QueryProofResponse_Pending{}, "pulsarchain/verification/Pending", nil)
	cdc.RegisterConcrete(&QueryProofResponse_FinalResult{}, "pulsarchain/verification/Final", nil)
}

func RegisterInterfaces(registrar codectypes.InterfaceRegistry) {
	registrar.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgSubmitProof{},
		&MsgUpdateParams{},
	)
	msgservice.RegisterMsgServiceDesc(registrar, &_Msg_serviceDesc)
}
