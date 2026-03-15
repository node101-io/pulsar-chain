package types

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	tx "github.com/cosmos/cosmos-sdk/types/tx"
)

// RegisterInterfaces registers the tx auth mode extension as a tx extension option.
func RegisterInterfaces(registrar codectypes.InterfaceRegistry) {
	registrar.RegisterImplementations(
		(*tx.TxExtensionOptionI)(nil),
		&TxAuthModeExtension{},
	)
}
