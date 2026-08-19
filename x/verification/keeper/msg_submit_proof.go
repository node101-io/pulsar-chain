package keeper

import (
	"context"
	"encoding/hex"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

// SubmitProof registers proof metadata under the next deterministic index at
// the current height. The proof bytes remain off-chain; only their hash and
// type become consensus state. This on-chain registration is the eligibility
// signal for sidecars: merely receiving a proof over P2P does not authorize a
// validator to vote on it.
func (m msgServer) SubmitProof(ctx context.Context, msg *types.MsgSubmitProof) (*types.MsgSubmitProofResponse, error) {
	if msg == nil || len(msg.ProofHash) != types.ProofHashSize {
		return nil, types.ErrInvalidProofHash
	}
	// Use a cache context so snapshot creation and all proof indexes are
	// committed together or not at all. A partial write could otherwise leave a
	// permanent hash reservation without a queryable proof, or a proof without
	// the snapshot needed to finalize it.
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
	// The first proof at a height freezes the validator set used for every vote
	// and the final two-thirds threshold for that height. Further submissions in
	// the same block reuse the snapshot even if staking state changes later.
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

	// The successful count is also the next index, so failed submissions never
	// create gaps in the canonical proof sequence. The one-byte index later
	// becomes part of the canonical leaf encoding.
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
