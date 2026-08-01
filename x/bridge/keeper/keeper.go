package keeper

import (
	"bytes"
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/node101-io/pulsar-chain/x/bridge/types"
)

const (
	BridgeStateItemName                = "bridge_state"
	ActionsReducedRootSnapshotsMapName = "actions_reduced_root_snapshots"
)

type Keeper struct {
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	Schema collections.Schema
	Params collections.Item[types.Params]

	BridgeState                 collections.Item[types.BridgeState]
	ActionsReducedRootSnapshots collections.Map[int64, []byte]

	bankKeeper           types.BankKeeper
	keyRegistryKeeper    types.KeyregistryKeeper
	archiveWrapperClient ArchiveWrapperQueryClient
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	bankKeeper types.BankKeeper,
	keyRegistryKeeper types.KeyregistryKeeper,
	archiveWrapperClient ArchiveWrapperQueryClient,
) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		addressCodec: addressCodec,
		authority:    bytes.Clone(authority),

		Params: collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		BridgeState: collections.NewItem(
			sb,
			types.BridgeStateKey,
			BridgeStateItemName,
			codec.CollValue[types.BridgeState](cdc),
		),
		ActionsReducedRootSnapshots: collections.NewMap(
			sb,
			types.ActionsReducedRootSnapshotsKey,
			ActionsReducedRootSnapshotsMapName,
			collections.Int64Key,
			collections.BytesValue,
		),

		bankKeeper:           bankKeeper,
		keyRegistryKeeper:    keyRegistryKeeper,
		archiveWrapperClient: archiveWrapperClient,
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
	return bytes.Clone(k.authority)
}

func (k Keeper) GetBridgeState(ctx context.Context) (types.BridgeState, error) {
	return k.BridgeState.Get(ctx)
}

func (k Keeper) GetLatestActionsReducedRoot(ctx context.Context) ([]byte, error) {
	bridgeState, err := k.GetBridgeState(ctx)
	if err != nil {
		return nil, err
	}

	if len(bridgeState.CurrentActionsReducedRoot) == 0 {
		return nil, types.ErrActionsReducedRootSnapshotNotFound
	}

	return bridgeState.CurrentActionsReducedRoot, nil
}

// GetActionsReducedRootAtHeight returns the newest snapshot at or before height.
func (k Keeper) GetActionsReducedRootAtHeight(ctx context.Context, height int64) ([]byte, error) {
	if height < 0 {
		return nil, types.ErrInvalidBridgeStateHeight
	}

	iter, err := k.ActionsReducedRootSnapshots.Iterate(
		ctx,
		(&collections.Range[int64]{}).EndInclusive(height).Descending(),
	)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	if !iter.Valid() {
		return nil, types.ErrActionsReducedRootSnapshotNotFound
	}

	return iter.Value()
}

// SetActionsReducedRoot stores the current root on BridgeState and also records
// it in the recent snapshot window used by consensus-time lookups.
func (k Keeper) SetActionsReducedRoot(ctx context.Context, height int64, root []byte) error {
	bridgeState, err := k.GetBridgeState(ctx)
	if err != nil {
		return err
	}

	bridgeState.CurrentActionsReducedRoot = root
	if err := k.BridgeState.Set(ctx, bridgeState); err != nil {
		return err
	}

	// Window size is a bridge param, so validators prune with the same value.
	params, err := k.Params.Get(ctx)
	if err != nil {
		return err
	}

	return k.setActionsReducedRootSnapshot(ctx, height, root, params.ActionsReducedRootSnapshotWindowSize)
}

// setActionsReducedRootSnapshot writes one height-root pair and prunes anything
// outside the configured rolling window.
func (k Keeper) setActionsReducedRootSnapshot(ctx context.Context, height int64, root []byte, windowSize int64) error {
	if height < 0 {
		return types.ErrInvalidBridgeStateHeight
	}
	if windowSize <= 0 {
		return types.ErrActionsReducedRootSnapshotWindowSizeMustBeGreaterThanZero
	}

	if err := k.ActionsReducedRootSnapshots.Set(ctx, height, root); err != nil {
		return err
	}

	return k.pruneActionsReducedRootSnapshots(ctx, windowSize)
}

func (k Keeper) pruneActionsReducedRootSnapshots(ctx context.Context, windowSize int64) error {
	iter, err := k.ActionsReducedRootSnapshots.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	heights := make([]int64, 0, int(windowSize)+1)
	for ; iter.Valid(); iter.Next() {
		height, err := iter.Key()
		if err != nil {
			return err
		}
		heights = append(heights, height)
	}

	// Heights are iterated in ascending order, so the front is the oldest entry.
	for int64(len(heights)) > windowSize {
		if err := k.ActionsReducedRootSnapshots.Remove(ctx, heights[0]); err != nil {
			return err
		}
		heights = heights[1:]
	}

	return nil
}
