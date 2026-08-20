package keeper

import (
	"context"
	"encoding/hex"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

// SubmitProof registers proof metadata under the next deterministic index at
// the current height. The proof material remains off-chain; its component
// hashes, proof family, and chain-derived verification ID become consensus
// state. This on-chain registration is the eligibility
// signal for sidecars: merely receiving a proof over P2P does not authorize a
// validator to vote on it.
func (m msgServer) SubmitProof(ctx context.Context, msg *types.MsgSubmitProof) (*types.MsgSubmitProofResponse, error) {
	if msg == nil {
		return nil, types.ErrInvalidProofHash
	}
	verificationID, err := types.ComputeVerificationID(
		msg.ProofType,
		msg.ProofHash,
		msg.PublicInputsHash,
		msg.VerificationKeyHash,
	)
	if err != nil {
		return nil, err
	}
	// Use a cache context so every proof index is committed together or not at
	// all. A partial write could otherwise leave a permanent ID reservation
	// without a queryable proof. Historical power is materialized once later in
	// EndBlock, after all proof transactions for this height have completed.
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	cacheCtx, write := sdkCtx.CacheContext()
	ctx = cacheCtx
	seen, err := m.SeenVerificationIDs.Has(ctx, verificationID[:])
	if err != nil {
		return nil, err
	}
	if seen {
		return nil, types.ErrDuplicateVerificationRequest
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
	// The successful count is also the next index, so failed submissions never
	// create gaps in the canonical proof sequence. The one-byte index later
	// becomes part of the canonical leaf encoding.
	key := types.NewProofStoreKey(height, count)
	record := types.ProofRecord{
		ProofHash:           append([]byte(nil), msg.ProofHash...),
		ProofType:           msg.ProofType,
		PublicInputsHash:    append([]byte(nil), msg.PublicInputsHash...),
		VerificationKeyHash: append([]byte(nil), msg.VerificationKeyHash...),
		VerificationId:      append([]byte(nil), verificationID[:]...),
	}
	if err := m.PendingProofs.Set(ctx, key, record); err != nil {
		return nil, err
	}
	if err := m.SeenVerificationIDs.Set(ctx, append([]byte(nil), verificationID[:]...), types.ProofKey{
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
		sdk.NewAttribute(types.AttributeKeyVerificationID, hex.EncodeToString(verificationID[:])),
		sdk.NewAttribute(types.AttributeKeyProofHash, hex.EncodeToString(msg.ProofHash)),
		sdk.NewAttribute(types.AttributeKeyPublicInputsHash, hex.EncodeToString(msg.PublicInputsHash)),
		sdk.NewAttribute(types.AttributeKeyVerificationKeyHash, hex.EncodeToString(msg.VerificationKeyHash)),
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
