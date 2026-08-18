package keeper

import (
	"context"
	"encoding/hex"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func (m msgServer) SubmitProof(ctx context.Context, msg *types.MsgSubmitProof) (*types.MsgSubmitProofResponse, error) {
	if msg == nil || len(msg.ProofHash) != types.ProofHashSize {
		return nil, types.ErrInvalidProofHash
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	cacheCtx, write := sdkCtx.CacheContext()
	ctx = cacheCtx
	seen, err := m.SeenProofHashes.Has(ctx, msg.ProofHash)
	if err != nil {
		return nil, err
	}
	if seen {
		return nil, types.ErrDuplicateProof
	}

	height, err := blockHeight(cacheCtx)
	if err != nil {
		return nil, err
	}
	params, err := m.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	count, err := m.proofCount(ctx, height)
	if err != nil {
		return nil, err
	}
	if count >= params.MaxProofsPerBlock {
		return nil, types.ErrMaxProofsPerBlock
	}
	if count == 0 {
		snapshotExists, err := m.ValidatorCountByHeight.Has(ctx, height)
		if err != nil {
			return nil, err
		}
		if !snapshotExists {
			if err := m.CreateValidatorSnapshot(ctx, height); err != nil {
				return nil, err
			}
		} else {
			validatorCount, err := m.ValidatorCountByHeight.Get(ctx, height)
			if err != nil || validatorCount == 0 {
				return nil, types.ErrEmptyValidatorSet
			}
		}
	}

	key := types.NewProofStoreKey(height, count)
	record := types.ProofRecord{ProofHash: append([]byte(nil), msg.ProofHash...), ProofType: msg.ProofType}
	if err := m.PendingProofs.Set(ctx, key, record); err != nil {
		return nil, err
	}
	if err := m.SeenProofHashes.Set(ctx, append([]byte(nil), msg.ProofHash...), types.ProofKey{
		SubmissionHeight: height,
		IndexInBlock:     count,
	}); err != nil {
		return nil, err
	}
	if err := m.ProofCountByHeight.Set(ctx, height, count+1); err != nil {
		return nil, err
	}
	if err := m.ProofTallies.Set(ctx, key, types.ProofTally{}); err != nil {
		return nil, err
	}

	cacheCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTypeProofSubmitted,
		sdk.NewAttribute(types.AttributeKeyProofHash, hex.EncodeToString(msg.ProofHash)),
		sdk.NewAttribute(types.AttributeKeySubmissionHeight, strconv.FormatUint(height, 10)),
		sdk.NewAttribute(types.AttributeKeyIndexInBlock, strconv.FormatUint(uint64(count), 10)),
		sdk.NewAttribute(types.AttributeKeyProofType, strconv.FormatUint(uint64(msg.ProofType), 10)),
	))
	write()

	return &types.MsgSubmitProofResponse{SubmissionHeight: height, IndexInBlock: count}, nil
}

func (k Keeper) proofCount(ctx context.Context, height uint64) (uint32, error) {
	found, err := k.ProofCountByHeight.Has(ctx, height)
	if err != nil || !found {
		return 0, err
	}

	return k.ProofCountByHeight.Get(ctx, height)
}
