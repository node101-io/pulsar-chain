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

func (k Keeper) HasVoteExt(ctx context.Context, index string) (bool, error) {
	return k.VoteExts.Has(ctx, index)
}

// RemoveVoteExt removes a voteExt from the store
func (k Keeper) RemoveVoteExt(ctx context.Context, index string) error {
	return k.VoteExts.Remove(ctx, index)
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
func (k Keeper) GetVoteExtsByHeight(ctx context.Context, height uint64) (list []*types.VoteExt) {
	_ = k.VoteExts.Walk(ctx, nil, func(_ string, val types.VoteExt) (stop bool, err error) {
		if val.Height == height {
			list = append(list, &val)
		}
		return false, nil
	})
	return
}
