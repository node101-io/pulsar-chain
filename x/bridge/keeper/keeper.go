package keeper

import (
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
	storeService corestore.KVStoreService
	cdc          codec.Codec
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
	return k.authority
}

func (k Keeper) GetBridgeState(ctx context.Context) (types.BridgeState, error) {
	return k.BridgeState.Get(ctx)
}

func (k Keeper) GetLatestActionsReducedRoot(ctx context.Context) ([]byte, error) {
	iter, err := k.ActionsReducedRootSnapshots.Iterate(
		ctx,
		(&collections.Range[int64]{}).Descending(),
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
