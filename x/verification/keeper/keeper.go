package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

const PendingProofsMapName string = "pending_proofs_map"

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	pendingProofs collections.Map[
		collections.Pair[int64, int64],
		[]byte,
	]

	Schema collections.Schema
	Params collections.Item[types.Params]
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

		pendingProofs: collections.NewMap(sb, types.PendingProofsKey, PendingProofsMapName,
			collections.PairKeyCodec(collections.Int64Key, collections.Int64Key), collections.BytesValue),
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

func (k Keeper) SetPendingProof(ctx context.Context, pendingProof []byte,
	blockHeight, pendingProofIndex int64) ([]byte, error) {

	key := collections.Join(blockHeight, pendingProofIndex)

	exists, err := k.pendingProofs.Has(ctx, key)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, types.ErrPendingProofAlreadyExists
	}

	if err := k.pendingProofs.Set(ctx, key, pendingProof); err != nil {
		return nil, err
	}

	if err := k.prunePendingProofs(ctx, blockHeight); err != nil {
		return nil, err
	}

	return nil, nil
}

func (k Keeper) GetPendingProof(ctx context.Context, blockHeight, pendingProofIndex int64) ([]byte, error) {
	return k.pendingProofs.Get(ctx, collections.Join(blockHeight, pendingProofIndex))
}

func (k Keeper) PendingProofExists(ctx context.Context, blockHeight, pendingProofIndex int64) (bool, error) {
	return k.pendingProofs.Has(ctx, collections.Join(blockHeight, pendingProofIndex))
}

func (k Keeper) IteratePendingProofs(ctx context.Context) (collections.Iterator[collections.Pair[int64, int64], []byte], error) {
	return k.pendingProofs.Iterate(ctx, nil)
}

func (k Keeper) prunePendingProofs(ctx context.Context, blockHeight int64) error {

	params, err := k.Params.Get(ctx)
	if err != nil {
		return err
	}

	pruneHeight := blockHeight - params.PendingProofBlocksWindowSize

	return k.pendingProofs.Clear(
		ctx,
		collections.NewPrefixedPairRange[int64, int64](pruneHeight),
	)
}
