package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
	minasignergo "github.com/node101-io/mina-signer-go/merklelist"
	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

const BridgeStateItemName string = "bridge_state"

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	Schema      collections.Schema
	Params      collections.Item[types.Params]
	BridgeState collections.Item[types.BridgeState]

	MerkleList *minasignergo.MerkleList

	bankKeeper        types.BankKeeper
	keyRegistryKeeper types.KeyregistryKeeper
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	bankKeeper types.BankKeeper,
	keyRegistryKeeper types.KeyregistryKeeper,
	merkleList *minasignergo.MerkleList,
) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		storeService: storeService,
		cdc:          cdc,
		addressCodec: addressCodec,
		authority:    authority,

		Params: collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		BridgeState: collections.NewItem(
			sb,
			types.BridgeStateKey,
			BridgeStateItemName,
			codec.CollValue[types.BridgeState](cdc),
		),
		bankKeeper:        bankKeeper,
		keyRegistryKeeper: keyRegistryKeeper,
		MerkleList:        merkleList,
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() []byte {
	return k.authority
}

func (k Keeper) GetBridgeState(ctx context.Context) (types.BridgeState, error) {
	return k.BridgeState.Get(ctx)
}

func (k Keeper) setBridgeState(ctx context.Context, st types.BridgeState) error {
	return k.BridgeState.Set(ctx, st)
}
