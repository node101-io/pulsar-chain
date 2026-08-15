package keeper

import (
	"bytes"
	"context"
	"fmt"
	"math"

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
		types.ProofID,
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
			newProofIDKeyCodec(), collections.BytesValue),
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

	key := types.ProofID{
		BlockHeight: blockHeight,
		ProofIndex:  pendingProofIndex,
	}

	if err := k.pendingProofs.Set(ctx, key, pendingProof); err != nil {
		return err
	}

	if err := k.prunePendingProofs(ctx, blockHeight); err != nil {
		return err
	}

	return nil
}

func (k Keeper) GetPendingProof(ctx context.Context, proofID types.ProofID) ([]byte, error) {
	return k.pendingProofs.Get(ctx, proofID)
}

func (k Keeper) PendingProofExists(ctx context.Context, proofID types.ProofID) (bool, error) {
	return k.pendingProofs.Has(ctx, proofID)
}

func (k Keeper) IteratePendingProofs(ctx context.Context) (collections.Iterator[types.ProofID, []byte], error) {
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
		pendingProofsByBlockHeightRange(pruneHeight),
	)
}

func (k Keeper) PendingProofBlockExists(
	ctx context.Context,
	blockHeight int64,
) (bool, error) {
	iter, err := k.pendingProofs.Iterate(
		ctx,
		pendingProofsByBlockHeightRange(blockHeight),
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
		pendingProofsByBlockHeightRange(blockHeight).Descending(),
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

	return key.ProofIndex + 1, nil
}

func (k Keeper) GetProofHashesByBlockHeight(
	ctx context.Context,
	blockHeight int64,
) ([][]byte, []types.ProofID, error) {
	iter, err := k.pendingProofs.Iterate(
		ctx,
		pendingProofsByBlockHeightRange(blockHeight),
	)
	if err != nil {
		return nil, nil, err
	}
	defer iter.Close()

	var hashes [][]byte
	var proofIDs []types.ProofID

	for ; iter.Valid(); iter.Next() {

		key, err := iter.Key()
		if err != nil {
			return nil, nil, err
		}

		value, err := iter.Value()
		if err != nil {
			return nil, nil, err
		}

		hashes = append(hashes, value)
		proofIDs = append(proofIDs, key)

	}
	return hashes, proofIDs, nil
}

func (k Keeper) ProofHashExists(ctx context.Context, proofHash []byte) (bool, error) {

	iter, err := k.IteratePendingProofs(ctx)
	if err != nil {
		return false, err
	}

	defer iter.Close()

	for ; iter.Valid(); iter.Next() {

		value, err := iter.Value()
		if err != nil {
			return false, err
		}

		if bytes.Equal(proofHash, value) {
			return true, nil
		}

	}
	return false, nil
}

func pendingProofsByBlockHeightRange(blockHeight int64) *collections.Range[types.ProofID] {
	return (&collections.Range[types.ProofID]{}).
		StartInclusive(types.ProofID{
			BlockHeight: blockHeight,
			ProofIndex:  math.MinInt64,
		}).
		EndInclusive(types.ProofID{
			BlockHeight: blockHeight,
			ProofIndex:  math.MaxInt64,
		})
}
