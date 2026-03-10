package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

const CosmosToMinaMapName string = "cosmos_to_mina"
const MinaToCosmosMapName string = "mina_to_cosmos"

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	Schema collections.Schema
	Params collections.Item[types.Params]

	cosmosToMina collections.Map[[]byte, []byte] // Cosmos PubKey --> Mina PubKey
	minaToCosmos collections.Map[[]byte, []byte] // Mina PubKey --> Cosmos PubKey
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,

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

		cosmosToMina: collections.NewMap(sb, types.CosmosToMinaPrefix, CosmosToMinaMapName, collections.BytesKey, collections.BytesValue),
		minaToCosmos: collections.NewMap(sb, types.MinaToCosmosPrefix, MinaToCosmosMapName, collections.BytesKey, collections.BytesValue),
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

func (k Keeper) SetCosmosToMina(ctx context.Context, cosmosPublicKey, minaPublicKey []byte) error {
	return k.cosmosToMina.Set(ctx, cosmosPublicKey, minaPublicKey)
}

func (k Keeper) GetCosmosToMina(ctx context.Context, cosmosPublicKey []byte) ([]byte, error) {
	return k.cosmosToMina.Get(ctx, cosmosPublicKey)
}

func (k Keeper) SetMinaToCosmos(ctx context.Context, minaPublicKey, cosmosPublicKey []byte) error {
	return k.minaToCosmos.Set(ctx, minaPublicKey, cosmosPublicKey)
}

func (k Keeper) GetMinaToCosmos(ctx context.Context, minaPublicKey []byte) ([]byte, error) {
	return k.minaToCosmos.Get(ctx, minaPublicKey)
}

func (k Keeper) CosmosToMinaHas(ctx context.Context, cosmosPublicKey []byte) (bool, error) {
	return k.cosmosToMina.Has(ctx, cosmosPublicKey)
}

func (k Keeper) MinaToCosmosHas(ctx context.Context, minaPublicKey []byte) (bool, error) {
	return k.minaToCosmos.Has(ctx, minaPublicKey)
}
