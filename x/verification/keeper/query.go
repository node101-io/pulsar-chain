package keeper

import (
	"bytes"
	"context"
	"math"

	"cosmossdk.io/collections"
	"github.com/cosmos/cosmos-sdk/types/query"

	"github.com/node101-io/pulsar-chain/x/verification/types"
)

// Params returns the current governance-controlled module parameters.
func (q queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	params, err := q.k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	return &types.QueryParamsResponse{Params: params}, nil
}

// Proof resolves one canonical proof key to either its pending record or its
// immutable final result. Returning a tagged union makes lifecycle movement
// explicit to clients instead of exposing two records for the same proof.
func (q queryServer) Proof(ctx context.Context, req *types.QueryProofRequest) (*types.QueryProofResponse, error) {
	if req == nil || req.IndexInBlock >= types.MaxVoteIndexExclusive {
		return nil, types.ErrInvalidVoteIndex
	}

	return q.proof(ctx, req.SubmissionHeight, req.IndexInBlock)
}

// ProofByHash uses the permanent hash registry, so it continues to work after
// pending proof state has been pruned. The resolved record is checked against
// the requested hash because a stale or mismatched reverse index is consensus
// corruption, not a normal not-found response.
func (q queryServer) ProofByHash(ctx context.Context, req *types.QueryProofByHashRequest) (*types.QueryProofResponse, error) {
	if req == nil || len(req.ProofHash) != types.ProofHashSize {
		return nil, types.ErrInvalidProofHash
	}
	exists, err := q.k.SeenProofHashes.Has(ctx, req.ProofHash)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, types.ErrProofNotFound
	}
	key, err := q.k.SeenProofHashes.Get(ctx, req.ProofHash)
	if err != nil {
		return nil, err
	}
	response, err := q.proof(ctx, key.SubmissionHeight, key.IndexInBlock)
	if err != nil {
		return nil, types.ErrProofStateCorrupted
	}
	if !bytes.Equal(proofResponseHash(response), req.ProofHash) {
		return nil, types.ErrProofStateCorrupted
	}

	return response, nil
}

func (q queryServer) proof(ctx context.Context, height uint64, index uint32) (*types.QueryProofResponse, error) {
	key := types.NewProofStoreKey(height, index)
	protoKey := types.ProofKey{SubmissionHeight: height, IndexInBlock: index}
	pending, err := q.k.PendingProofs.Has(ctx, key)
	if err != nil {
		return nil, err
	}
	final, err := q.k.FinalProofResults.Has(ctx, key)
	if err != nil {
		return nil, err
	}
	// A proof moves from PendingProofs to FinalProofResults at finalization; it
	// must never exist in both lifecycle stores. Detecting this in queries avoids
	// presenting an arbitrary state when the underlying invariant is broken.
	if pending && final {
		return nil, types.ErrProofStateCorrupted
	}
	if pending {
		record, err := q.k.PendingProofs.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if err := q.validateRegisteredProof(ctx, key, record.ProofHash); err != nil {
			return nil, err
		}
		return &types.QueryProofResponse{
			ProofKey: protoKey,
			State:    &types.QueryProofResponse_Pending{Pending: &record},
		}, nil
	}
	if final {
		result, err := q.k.FinalProofResults.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if err := validateFinalProofResult(key, result); err != nil {
			return nil, err
		}
		if err := q.validateRegisteredProof(ctx, key, result.ProofHash); err != nil {
			return nil, err
		}
		return &types.QueryProofResponse{
			ProofKey: protoKey,
			State:    &types.QueryProofResponse_FinalResult{FinalResult: &result},
		}, nil
	}

	return nil, types.ErrProofNotFound
}

// ProofsByHeight returns the pending set before finalization and the final set
// afterwards. A height cannot legitimately contain a mixture of both states
// because finalization and pruning are atomic for the whole proof height.
func (q queryServer) ProofsByHeight(ctx context.Context, req *types.QueryProofsByHeightRequest) (*types.QueryProofsByHeightResponse, error) {
	if req == nil {
		return nil, types.ErrProofHeightNotFound
	}
	pending, err := q.k.ProofCountByHeight.Has(ctx, req.SubmissionHeight)
	if err != nil {
		return nil, err
	}
	if pending {
		count, err := q.k.ProofCountByHeight.Get(ctx, req.SubmissionHeight)
		if err != nil {
			return nil, err
		}
		if count == 0 || count > types.MaxVoteIndexExclusive {
			return nil, types.ErrProofStateCorrupted
		}
		proofs, page, err := query.CollectionPaginate(
			ctx,
			q.k.PendingProofs,
			req.Pagination,
			func(key types.ProofStoreKey, value types.ProofRecord) (types.QueryProofResponse, error) {
				if err := q.validateRegisteredProof(ctx, key, value.ProofHash); err != nil {
					return types.QueryProofResponse{}, err
				}
				return types.QueryProofResponse{
					ProofKey: types.ProofKey{SubmissionHeight: key.K1(), IndexInBlock: key.K2()},
					State:    &types.QueryProofResponse_Pending{Pending: &value},
				}, nil
			},
			query.WithCollectionPaginationPairPrefix[uint64, uint32](req.SubmissionHeight),
		)
		if err != nil {
			return nil, err
		}
		return &types.QueryProofsByHeightResponse{Proofs: proofs, Pagination: page}, nil
	}

	proofs, page, err := query.CollectionPaginate(
		ctx,
		q.k.FinalProofResults,
		req.Pagination,
		func(key types.ProofStoreKey, value types.FinalProofResult) (types.QueryProofResponse, error) {
			if err := validateFinalProofResult(key, value); err != nil {
				return types.QueryProofResponse{}, err
			}
			if err := q.validateRegisteredProof(ctx, key, value.ProofHash); err != nil {
				return types.QueryProofResponse{}, err
			}
			return types.QueryProofResponse{
				ProofKey: types.ProofKey{SubmissionHeight: key.K1(), IndexInBlock: key.K2()},
				State:    &types.QueryProofResponse_FinalResult{FinalResult: &value},
			}, nil
		},
		query.WithCollectionPaginationPairPrefix[uint64, uint32](req.SubmissionHeight),
	)
	if err != nil {
		return nil, err
	}

	return &types.QueryProofsByHeightResponse{Proofs: proofs, Pagination: page}, nil
}

// Commitment returns one retained validator commitment by operator address and
// commitment height.
func (q queryServer) Commitment(ctx context.Context, req *types.QueryCommitmentRequest) (*types.QueryCommitmentResponse, error) {
	if req == nil {
		return nil, types.ErrCommitmentNotFound
	}
	validator, err := q.k.stakingKeeper.ValidatorAddressCodec().StringToBytes(req.Validator)
	if err != nil {
		return nil, types.ErrInvalidValidator
	}
	key := types.NewCommitmentStoreKey(validator, req.CommitmentHeight)
	exists, err := q.k.Commitments.Has(ctx, key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, types.ErrCommitmentNotFound
	}
	commitment, err := q.k.Commitments.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if len(commitment) != types.CommitmentHashSize {
		return nil, types.ErrProofStateCorrupted
	}

	return &types.QueryCommitmentResponse{Commitment: types.CommitmentEntry{
		Validator:        req.Validator,
		CommitmentHeight: req.CommitmentHeight,
		Commitment:       types.CommitmentRecord{Commitment: commitment},
	}}, nil
}

// CommitmentsByValidator lists the currently retained commitments for one
// validator. Old commitments disappear after their final reveal window.
func (q queryServer) CommitmentsByValidator(ctx context.Context, req *types.QueryCommitmentsByValidatorRequest) (*types.QueryCommitmentsByValidatorResponse, error) {
	if req == nil {
		return nil, types.ErrInvalidValidator
	}
	validator, err := q.k.stakingKeeper.ValidatorAddressCodec().StringToBytes(req.Validator)
	if err != nil {
		return nil, types.ErrInvalidValidator
	}
	commitments, page, err := query.CollectionPaginate(
		ctx,
		q.k.Commitments,
		req.Pagination,
		func(key types.CommitmentStoreKey, value []byte) (types.CommitmentEntry, error) {
			if len(value) != types.CommitmentHashSize {
				return types.CommitmentEntry{}, types.ErrProofStateCorrupted
			}
			return types.CommitmentEntry{
				Validator:        req.Validator,
				CommitmentHeight: key.K2(),
				Commitment:       types.CommitmentRecord{Commitment: value},
			}, nil
		},
		query.WithCollectionPaginationPairPrefix[[]byte, uint64](validator),
	)
	if err != nil {
		return nil, err
	}

	return &types.QueryCommitmentsByValidatorResponse{Commitments: commitments, Pagination: page}, nil
}

// Vote returns a validator's effective state for one proof. Equivocated votes
// remain observable but contribute to neither tally. Keeping the marker makes
// the removal auditable and prevents a later reveal from restoring a vote.
func (q queryServer) Vote(ctx context.Context, req *types.QueryVoteRequest) (*types.QueryVoteResponse, error) {
	if req == nil || req.IndexInBlock >= types.MaxVoteIndexExclusive {
		return nil, types.ErrInvalidVoteIndex
	}
	validator, err := q.k.stakingKeeper.ValidatorAddressCodec().StringToBytes(req.Validator)
	if err != nil {
		return nil, types.ErrInvalidValidator
	}
	key := types.NewVerificationVoteStoreKey(req.SubmissionHeight, req.IndexInBlock, validator)
	exists, err := q.k.VerificationVotes.Has(ctx, key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, types.ErrProofNotFound
	}
	value, err := q.k.VerificationVotes.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	state := types.VoteState(value)
	if state <= types.VoteState_VOTE_STATE_NONE || state > types.VoteState_VOTE_STATE_EQUIVOCATED {
		return nil, types.ErrProofStateCorrupted
	}

	return &types.QueryVoteResponse{Vote: types.VerificationVoteEntry{
		ProofKey:  types.ProofKey{SubmissionHeight: req.SubmissionHeight, IndexInBlock: req.IndexInBlock},
		Validator: req.Validator,
		State:     state,
	}}, nil
}

// VerificationVotes lists all recorded effective validator states for a proof.
func (q queryServer) VerificationVotes(ctx context.Context, req *types.QueryVerificationVotesRequest) (*types.QueryVerificationVotesResponse, error) {
	if req == nil || req.IndexInBlock >= types.MaxVoteIndexExclusive {
		return nil, types.ErrInvalidVoteIndex
	}
	prefix := collections.TripleSuperPrefix[uint64, uint32, []byte](req.SubmissionHeight, req.IndexInBlock)
	votes, page, err := query.CollectionPaginate(
		ctx,
		q.k.VerificationVotes,
		req.Pagination,
		func(key types.VerificationVoteStoreKey, value uint32) (types.VerificationVoteEntry, error) {
			state := types.VoteState(value)
			if state <= types.VoteState_VOTE_STATE_NONE || state > types.VoteState_VOTE_STATE_EQUIVOCATED {
				return types.VerificationVoteEntry{}, types.ErrProofStateCorrupted
			}
			validator, err := q.k.stakingKeeper.ValidatorAddressCodec().BytesToString(key.K3())
			if err != nil {
				return types.VerificationVoteEntry{}, err
			}
			return types.VerificationVoteEntry{
				ProofKey:  types.ProofKey{SubmissionHeight: key.K1(), IndexInBlock: key.K2()},
				Validator: validator,
				State:     state,
			}, nil
		},
		func(options *query.CollectionsPaginateOptions[types.VerificationVoteStoreKey]) {
			options.Prefix = &prefix
		},
	)
	if err != nil {
		return nil, err
	}

	return &types.QueryVerificationVotesResponse{Votes: votes, Pagination: page}, nil
}

// ProofTally returns the live effective tally for a pending proof.
func (q queryServer) ProofTally(ctx context.Context, req *types.QueryProofTallyRequest) (*types.QueryProofTallyResponse, error) {
	if req == nil || req.IndexInBlock >= types.MaxVoteIndexExclusive {
		return nil, types.ErrInvalidVoteIndex
	}
	key := types.NewProofStoreKey(req.SubmissionHeight, req.IndexInBlock)
	exists, err := q.k.ProofTallies.Has(ctx, key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, types.ErrProofNotFound
	}
	tally, err := q.k.ProofTallies.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	totalPower, err := q.k.TotalVotingPowerByHeight.Get(ctx, req.SubmissionHeight)
	if err != nil || validatePowerTally(tally, totalPower) != nil {
		return nil, types.ErrProofStateCorrupted
	}

	return &types.QueryProofTallyResponse{
		ProofKey: types.ProofKey{SubmissionHeight: req.SubmissionHeight, IndexInBlock: req.IndexInBlock},
		Tally:    tally,
	}, nil
}

// FinalProofResult returns an immutable result and revalidates that its stored
// threshold, tally, status, and permanent hash mapping are mutually consistent.
// Queries therefore never normalize or hide corrupted consensus state.
func (q queryServer) FinalProofResult(ctx context.Context, req *types.QueryFinalProofResultRequest) (*types.QueryFinalProofResultResponse, error) {
	if req == nil || req.IndexInBlock >= types.MaxVoteIndexExclusive {
		return nil, types.ErrInvalidVoteIndex
	}
	key := types.NewProofStoreKey(req.SubmissionHeight, req.IndexInBlock)
	exists, err := q.k.FinalProofResults.Has(ctx, key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, types.ErrProofNotFound
	}
	result, err := q.k.FinalProofResults.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if err := validateFinalProofResult(key, result); err != nil {
		return nil, err
	}
	if err := q.validateRegisteredProof(ctx, key, result.ProofHash); err != nil {
		return nil, err
	}

	return &types.QueryFinalProofResultResponse{
		ProofKey:    types.ProofKey{SubmissionHeight: req.SubmissionHeight, IndexInBlock: req.IndexInBlock},
		FinalResult: result,
	}, nil
}

func (q queryServer) validateRegisteredProof(ctx context.Context, key types.ProofStoreKey, proofHash []byte) error {
	if len(proofHash) != types.ProofHashSize {
		return types.ErrProofStateCorrupted
	}
	registered, err := q.k.SeenProofHashes.Get(ctx, proofHash)
	if err != nil {
		return types.ErrProofStateCorrupted
	}
	if registered.SubmissionHeight != key.K1() || registered.IndexInBlock != key.K2() {
		return types.ErrProofStateCorrupted
	}

	return nil
}

func validateFinalProofResult(key types.ProofStoreKey, result types.FinalProofResult) error {
	if len(result.ProofHash) != types.ProofHashSize || result.SubmissionHeight != key.K1() {
		return types.ErrProofStateCorrupted
	}
	if result.SubmissionHeight > math.MaxUint64-(types.VerificationLifetime-1) ||
		result.FinalizedHeight != result.SubmissionHeight+types.VerificationLifetime-1 {
		return types.ErrProofStateCorrupted
	}
	threshold, err := types.ComputeVotingPowerThreshold(result.TotalVotingPower)
	if err != nil || result.VotingPowerThreshold != threshold {
		return types.ErrProofStateCorrupted
	}
	if validatePowerTally(types.ProofTally{
		ValidVotingPower:   result.ValidVotingPower,
		InvalidVotingPower: result.InvalidVotingPower,
	}, result.TotalVotingPower) != nil {
		return types.ErrProofStateCorrupted
	}
	expected := types.ProofStatus_PROOF_STATUS_INCONCLUSIVE
	if types.HasTwoThirdsMajority(result.ValidVotingPower, result.TotalVotingPower) {
		expected = types.ProofStatus_PROOF_STATUS_VALID
	} else if types.HasTwoThirdsMajority(result.InvalidVotingPower, result.TotalVotingPower) {
		expected = types.ProofStatus_PROOF_STATUS_INVALID
	}
	if result.Status != expected {
		return types.ErrProofStateCorrupted
	}

	return nil
}

func proofResponseHash(response *types.QueryProofResponse) []byte {
	if response == nil {
		return nil
	}
	if pending := response.GetPending(); pending != nil {
		return pending.ProofHash
	}
	if final := response.GetFinalResult(); final != nil {
		return final.ProofHash
	}
	return nil
}
