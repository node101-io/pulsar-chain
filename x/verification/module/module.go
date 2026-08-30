package verification

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/appmodule"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"

	"github.com/node101-io/pulsar-chain/x/verification/keeper"
	"github.com/node101-io/pulsar-chain/x/verification/types"
)

var (
	_ module.AppModuleBasic   = (*AppModule)(nil)
	_ module.HasGenesis       = (*AppModule)(nil)
	_ appmodule.AppModule     = (*AppModule)(nil)
	_ appmodule.HasEndBlocker = (*AppModule)(nil)
)

// AppModule connects the replicated verification keeper to the Cosmos SDK
// lifecycle and service registry. Validator-local sidecar calls and commitment
// salts deliberately remain outside this module wrapper; only deterministic
// proof, commitment, vote, tally, and final-result state is exported at genesis.
type AppModule struct {
	c      codec.Codec
	keeper keeper.Keeper
}

// NewAppModule creates the Cosmos SDK verification module wrapper.
func NewAppModule(c codec.Codec, k keeper.Keeper) AppModule {
	return AppModule{c: c, keeper: k}
}

// IsAppModule marks AppModule as a core application module.
func (AppModule) IsAppModule() {}

// Name returns the verification module name.
func (AppModule) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec registers verification oneof variants.
func (AppModule) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterLegacyAminoCodec(cdc)
}

// RegisterGRPCGatewayRoutes exposes deterministic consensus-state queries over
// REST. It does not proxy local sidecar status because two validators may observe
// different QUEUED or VERIFYING states without disagreeing on chain state.
func (AppModule) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	if err := types.RegisterQueryHandlerClient(clientCtx.CmdContext, mux, types.NewQueryClient(clientCtx)); err != nil {
		panic(err)
	}
}

// RegisterInterfaces registers verification SDK messages.
func (AppModule) RegisterInterfaces(registrar codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(registrar)
}

// RegisterServices registers proof submission as the user-facing Msg service
// and deterministic reads as Query service. Validator commitments and
// revelations arrive through vote extensions, not public SDK transactions.
func (am AppModule) RegisterServices(registrar grpc.ServiceRegistrar) error {
	types.RegisterMsgServer(registrar, keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(registrar, keeper.NewQueryServerImpl(am.keeper))

	return nil
}

// DefaultGenesis returns the default serialized verification state.
func (am AppModule) DefaultGenesis(codec.JSONCodec) json.RawMessage {
	return am.c.MustMarshalJSON(types.DefaultGenesis())
}

// ValidateGenesis checks the serialized state and its cross-store invariants.
func (am AppModule) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, raw json.RawMessage) error {
	var state types.GenesisState
	if err := am.c.UnmarshalJSON(raw, &state); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}

	return state.Validate()
}

// InitGenesis loads validated verification consensus state.
func (am AppModule) InitGenesis(ctx sdk.Context, _ codec.JSONCodec, raw json.RawMessage) {
	var state types.GenesisState
	if err := am.c.UnmarshalJSON(raw, &state); err != nil {
		panic(fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err))
	}
	if err := am.keeper.InitGenesis(ctx, state); err != nil {
		panic(fmt.Errorf("failed to initialize %s genesis state: %w", types.ModuleName, err))
	}
}

// ExportGenesis serializes all verification consensus state. Local journals,
// salts, and sidecar progress are intentionally absent because they are private
// operational state and must not be copied into the shared genesis app hash.
func (am AppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage {
	state, err := am.keeper.ExportGenesis(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to export %s genesis state: %w", types.ModuleName, err))
	}
	raw, err := am.c.MarshalJSON(state)
	if err != nil {
		panic(fmt.Errorf("failed to marshal %s genesis state: %w", types.ModuleName, err))
	}

	return raw
}

// ConsensusVersion returns the module migration version.
func (AppModule) ConsensusVersion() uint64 { return 1 }

// EndBlock finalizes proof heights whose H+5 reveal window has closed and prunes
// transient snapshots, commitments, and votes. Final results and the permanent
// verification-ID registry remain queryable after active lifecycle state is removed.
func (am AppModule) EndBlock(ctx context.Context) error {
	return am.keeper.EndBlock(ctx)
}
