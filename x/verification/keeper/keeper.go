package keeper

import (
	"bytes"
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/errors"
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

func (k Keeper) AppendPendingProof(ctx context.Context, pendingProof []byte,
	blockHeight int64) error {

	params, err := k.Params.Get(ctx)
	if err != nil {
		return err
	}

	pendingProofIndex, err := k.GetNextPendingProofIndex(ctx, blockHeight)
	if err != nil {
		return err
	}

	// MaxProofRange limits how many zero-based pending proof indexes a block may contain.
	if pendingProofIndex >= params.MaxProofRange {
		return errors.Wrap(
			types.ErrFailedToAppendPendingProof,
			"number of proof limit has been reached",
		)
	}

	key := collections.Join(blockHeight, pendingProofIndex)

	if err := k.pendingProofs.Set(ctx, key, pendingProof); err != nil {
		return err
	}

	if err := k.prunePendingProofs(ctx, blockHeight); err != nil {
		return err
	}

	return nil
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

	exists, err := k.PendingProofBlockExists(ctx, pruneHeight)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	return k.pendingProofs.Clear(
		ctx,
		collections.NewPrefixedPairRange[int64, int64](pruneHeight),
	)
}

func (k Keeper) PendingProofBlockExists(
	ctx context.Context,
	blockHeight int64,
) (bool, error) {
	iter, err := k.pendingProofs.Iterate(
		ctx,
		collections.NewPrefixedPairRange[int64, int64](blockHeight),
	)
	if err != nil {
		return false, err
	}
	defer iter.Close()

	return iter.Valid(), nil
}

func (k Keeper) GetNextPendingProofIndex(
	ctx context.Context,
	blockHeight int64,
) (int64, error) {
	iter, err := k.pendingProofs.Iterate(
		ctx,
		collections.NewPrefixedPairRange[int64, int64](blockHeight).
			Descending(),
	)
	if err != nil {
		return 0, err
	}
	defer iter.Close()

	if !iter.Valid() {
		return 0, nil
	}

	key, err := iter.Key()
	if err != nil {
		return 0, err
	}

	return key.K2() + 1, nil
}

func (k Keeper) GetProofIDByProofHash(ctx context.Context, proofHash []byte) (int64, error) {

	iter, err := k.IteratePendingProofs(ctx)
	if err != nil {
		return 0, err
	}

	defer iter.Close()

	for ; iter.Valid(); iter.Next() {

		pair, err := iter.Key()
		if err != nil {
			return 0, err
		}

		height, index := pair.K1(), pair.K2()

		proof, err := k.GetPendingProof(ctx, height, index)
		if err != nil {
			return 0, err
		}

		if bytes.Equal(proof, proofHash) {
			return index, nil
		}
	}

	return 0, fmt.Errorf("pending proof not found")
}

func (k Keeper) GetParams(ctx context.Context) (types.Params, error) {
	return k.Params.Get(ctx)
}
