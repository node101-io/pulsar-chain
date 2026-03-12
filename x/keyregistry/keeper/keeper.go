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

const UserCosmosToMinaMapName string = "user_cosmos_to_mina"
const UserMinaToCosmosMapName string = "user_mina_to_cosmos"

const ValidatorCosmosToMinaMapName string = "validator_cosmos_to_mina"
const ValidatorMinaToCosmosMapName string = "validator_mina_to_cosmos"

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	Schema collections.Schema
	Params collections.Item[types.Params]

	userCosmosToMina collections.Map[[]byte, []byte] // Cosmos PubKey --> Mina PubKey
	userMinaToCosmos collections.Map[[]byte, []byte] // Mina PubKey --> Cosmos PubKey

	validatorCosmosToMina collections.Map[[]byte, []byte] // Validator Cosmos PubKey --> Validator Mina PubKey
	validatorMinaToCosmos collections.Map[[]byte, []byte] // Validator Mina PubKey --> Validator Cosmos PubKey
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

		userCosmosToMina: collections.NewMap(sb, types.UserCosmosToMinaPrefix, UserCosmosToMinaMapName, collections.BytesKey, collections.BytesValue),
		userMinaToCosmos: collections.NewMap(sb, types.UserMinaToCosmosPrefix, UserMinaToCosmosMapName, collections.BytesKey, collections.BytesValue),

		validatorCosmosToMina: collections.NewMap(sb, types.ValidatorCosmosToMinaPrefix, ValidatorCosmosToMinaMapName, collections.BytesKey, collections.BytesValue),
		validatorMinaToCosmos: collections.NewMap(sb, types.ValidatorMinaToCosmosPrefix, ValidatorMinaToCosmosMapName, collections.BytesKey, collections.BytesValue),
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

func (k Keeper) UserSetCosmosToMina(ctx context.Context, cosmosPublicKey, minaPublicKey []byte) error {
	return k.userCosmosToMina.Set(ctx, cosmosPublicKey, minaPublicKey)
}

func (k Keeper) UserGetCosmosToMina(ctx context.Context, cosmosPublicKey []byte) ([]byte, error) {
	return k.userCosmosToMina.Get(ctx, cosmosPublicKey)
}

func (k Keeper) UserSetMinaToCosmos(ctx context.Context, minaPublicKey, cosmosPublicKey []byte) error {
	return k.userMinaToCosmos.Set(ctx, minaPublicKey, cosmosPublicKey)
}

func (k Keeper) UserGetMinaToCosmos(ctx context.Context, minaPublicKey []byte) ([]byte, error) {
	return k.userMinaToCosmos.Get(ctx, minaPublicKey)
}

func (k Keeper) UserCosmosToMinaHas(ctx context.Context, cosmosPublicKey []byte) (bool, error) {
	return k.userCosmosToMina.Has(ctx, cosmosPublicKey)
}

func (k Keeper) UserMinaToCosmosHas(ctx context.Context, minaPublicKey []byte) (bool, error) {
	return k.userMinaToCosmos.Has(ctx, minaPublicKey)
}

func (k Keeper) ValidatorSetCosmosToMina(ctx context.Context, cosmosPublicKey, minaPublicKey []byte) error {
	return k.validatorCosmosToMina.Set(ctx, cosmosPublicKey, minaPublicKey)
}

func (k Keeper) ValidatorGetCosmosToMina(ctx context.Context, cosmosPublicKey []byte) ([]byte, error) {
	return k.validatorCosmosToMina.Get(ctx, cosmosPublicKey)
}

func (k Keeper) ValidatorSetMinaToCosmos(ctx context.Context, minaPublicKey, cosmosPublicKey []byte) error {
	return k.validatorMinaToCosmos.Set(ctx, minaPublicKey, cosmosPublicKey)
}

func (k Keeper) ValidatorGetMinaToCosmos(ctx context.Context, minaPublicKey []byte) ([]byte, error) {
	return k.validatorMinaToCosmos.Get(ctx, minaPublicKey)
}

func (k Keeper) ValidatorCosmosToMinaHas(ctx context.Context, cosmosPublicKey []byte) (bool, error) {
	return k.validatorCosmosToMina.Has(ctx, cosmosPublicKey)
}

func (k Keeper) ValidatorMinaToCosmosHas(ctx context.Context, minaPublicKey []byte) (bool, error) {
	return k.validatorMinaToCosmos.Has(ctx, minaPublicKey)
}
