package keeper

import (
	"context"

	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

// SetVoteExt set a specific voteExt in the store from its index
func (k Keeper) SetVoteExt(ctx context.Context, voteExt types.VoteExt) error {
	err := k.VoteExts.Set(ctx, voteExt.Index, voteExt)
	if err != nil {
		return err
	}
	return nil
}

// GetVoteExt returns a voteExt from its index
func (k Keeper) GetVoteExt(ctx context.Context, index string) (types.VoteExt, error) {
	return k.VoteExts.Get(ctx, index)
}

// HasVoteExt returs te existence of a voteExt from its index
func (k Keeper) HasVoteExt(ctx context.Context, index string) (bool, error) {
	return k.VoteExts.Has(ctx, index)
}

// RemoveVoteExt removes a voteExt from the store
func (k Keeper) RemoveVoteExt(ctx context.Context, index string) error {
	return k.VoteExts.Remove(ctx, index)
}

// RemoveAllVoteExts removes all vote extensions from collection map
func (k Keeper) RemoveAllVoteExts(ctx context.Context) error {
	err := k.VoteExts.Walk(ctx, nil, func(_ string, val types.VoteExt) (stop bool, err error) {
		err = k.VoteExts.Remove(ctx, val.Index)
		if err != nil {
			return true, err
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	return nil
}

// GetAllVoteExt returns all voteExt
func (k Keeper) GetAllVoteExt(ctx context.Context) (list []*types.VoteExt, err error) {
	err = k.VoteExts.Walk(ctx, nil, func(_ string, val types.VoteExt) (stop bool, err error) {
		list = append(list, &val)
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return
}

// GetVoteExtsByHeight returns the vote extensions, given a block height
func (k Keeper) GetVoteExtsByHeight(ctx context.Context, height uint64) (list []*types.VoteExt) {
	_ = k.VoteExts.Walk(ctx, nil, func(_ string, val types.VoteExt) (stop bool, err error) {
		if val.Height == height {
			list = append(list, &val)
		}
		return false, nil
	})
	return
}
