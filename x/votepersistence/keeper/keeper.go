package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

const VoteStorageMapName string = "vote_storage"

type Keeper struct {
	storeService      corestore.KVStoreService
	cdc               codec.Codec
	addressCodec      address.Codec
	stakingKeeper     types.StakingKeeper
	keyregistryKeeper types.KeyregistryKeeper
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	voteStorage collections.Map[collections.Pair[int64, []byte], []byte]

	Schema collections.Schema
	Params collections.Item[types.Params]
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	stakingKeeper types.StakingKeeper,
	keyregistryKeeper types.KeyregistryKeeper,
	authority []byte,

) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		storeService:      storeService,
		cdc:               cdc,
		addressCodec:      addressCodec,
		stakingKeeper:     stakingKeeper,
		keyregistryKeeper: keyregistryKeeper,
		authority:         authority,

		Params: collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),

		voteStorage: collections.NewMap(sb,
			types.VoteStorageMapPrefix,
			VoteStorageMapName,
			collections.PairKeyCodec(collections.Int64Key, collections.BytesKey),
			collections.BytesValue),
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

func (k Keeper) SetVote(ctx context.Context, blockHeight int64, minaAddress, voteExtensions []byte) error {
	return k.voteStorage.Set(ctx, collections.Join(blockHeight, minaAddress), voteExtensions)
}
func (k Keeper) GetVote(ctx context.Context, blockHeight int64, minaAddress []byte) ([]byte, error) {
	return k.voteStorage.Get(ctx, collections.Join(blockHeight, minaAddress))
}
func (k Keeper) RemoveVote(ctx context.Context, blockHeight int64, minaAddress []byte) error {
	return k.voteStorage.Remove(ctx, collections.Join(blockHeight, minaAddress))
}

func (k Keeper) RemoveVotes(ctx context.Context) error {

	iterator, err := k.voteStorage.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer iterator.Close()

	for iterator.Valid() {
		key, err := iterator.Key()
		if err != nil {
			return err
		}
		err = k.voteStorage.Remove(ctx, key)
		if err != nil {
			return err
		}
		iterator.Next()
	}
	return nil
}

func (k Keeper) VoteExists(ctx context.Context, blockHeight int64, minaAddress []byte) (bool, error) {
	return k.voteStorage.Has(ctx, collections.Join(blockHeight, minaAddress))
}
func (k Keeper) IterateVotes(ctx context.Context) (collections.Iterator[collections.Pair[int64, []byte], []byte], error) {
	return k.voteStorage.Iterate(ctx, nil)
}
