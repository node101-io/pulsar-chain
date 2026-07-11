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
	storeService  corestore.KVStoreService
	cdc           codec.Codec
	addressCodec  address.Codec
	stakingKeeper types.StakingKeeper
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	Schema collections.Schema
	Params collections.Item[types.Params]

	userCosmosToMina collections.Map[[]byte, []byte] // Cosmos Public Key --> Mina Public Key
	userMinaToCosmos collections.Map[[]byte, []byte] // Mina Public Key --> CosmosPublic Key

	validatorCosmosToMina collections.Map[[]byte, []byte] // Validator Cosmos Public Key  --> Validator Mina Public Key
	validatorMinaToCosmos collections.Map[[]byte, []byte] // Validator Mina Public Key  --> Validator Cosmos Public Key
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	stakingKeeper types.StakingKeeper,
	authority []byte,

) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		storeService:  storeService,
		cdc:           cdc,
		addressCodec:  addressCodec,
		stakingKeeper: stakingKeeper,
		authority:     authority,

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

func (k Keeper) UserGetCosmosToMina(ctx context.Context, cosmosPubKey []byte) ([]byte, error) {
	return k.userCosmosToMina.Get(ctx, cosmosPubKey)
}

func (k Keeper) UserGetMinaToCosmos(ctx context.Context, minaPubKey []byte) ([]byte, error) {
	return k.userMinaToCosmos.Get(ctx, minaPubKey)
}

func (k Keeper) UserCosmosToMinaHas(ctx context.Context, cosmosPubKey []byte) (bool, error) {
	return k.userCosmosToMina.Has(ctx, cosmosPubKey)
}

func (k Keeper) UserMinaToCosmosHas(ctx context.Context, minaPubKey []byte) (bool, error) {
	return k.userMinaToCosmos.Has(ctx, minaPubKey)
}

func (k Keeper) ValidatorGetCosmosToMina(ctx context.Context, consensusPubKey []byte) ([]byte, error) {
	return k.validatorCosmosToMina.Get(ctx, consensusPubKey)
}

func (k Keeper) ValidatorGetMinaToCosmos(ctx context.Context, minaPubKey []byte) ([]byte, error) {
	return k.validatorMinaToCosmos.Get(ctx, minaPubKey)
}

func (k Keeper) ValidatorCosmosToMinaHas(ctx context.Context, consensusPubKey []byte) (bool, error) {
	return k.validatorCosmosToMina.Has(ctx, consensusPubKey)
}

func (k Keeper) ValidatorMinaToCosmosHas(ctx context.Context, minaPubKey []byte) (bool, error) {
	return k.validatorMinaToCosmos.Has(ctx, minaPubKey)
}
