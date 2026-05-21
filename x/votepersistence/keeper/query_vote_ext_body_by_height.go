package keeper

import (
	"context"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/pulsar-chain/x/votepersistence/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const ActionsReducedRoot string = "pulsar"

func (q queryServer) VoteExtBodyByHeight(ctx context.Context, req *types.QueryVoteExtBodyByHeightRequest) (*types.QueryVoteExtBodyByHeightResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.BlockHeight < 2 {
		return nil, status.Error(codes.InvalidArgument, "there is no vote extension body for heights smaller than 2")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	if req.BlockHeight >= sdkCtx.BlockHeight() {
		return nil, status.Error(codes.InvalidArgument, "no vote extensions in the requested block yet")
	}

	if q.k.stakingKeeper == nil || q.k.keyregistryKeeper == nil {
		return nil, status.Error(codes.Internal, "vote extension query dependencies are not configured")
	}

	voteExtBody, err := q.constructVoteExtBodyByHeight(ctx, req.BlockHeight)
	if err != nil {
		return nil, err
	}

	return &types.QueryVoteExtBodyByHeightResponse{
		VoteExtBody: voteExtBody,
	}, nil
}

func (q queryServer) constructVoteExtBodyByHeight(ctx context.Context, voteExtensionHeight int64) (*types.VoteExtBody, error) {
	// VoteExtBodyByHeight reconstructs the same body produced by ExtendVote(N).
	// The request height is the vote-extension consensus height N, while the body
	// signs the transition from state N-2 to state N-1.
	signedStateHeight := voteExtensionHeight - 2
	// HistoricalInfo(H).Header.AppHash is the state root after H-1, so H=N-1
	// yields the source state root for N-2.
	stateRootHistoricalInfoHeight := voteExtensionHeight - 1
	// HistoricalInfo(N).Valset is the validator set after the signed transition
	// and therefore matches NextValidatorSetHash in the body.
	targetValidatorSetHistoricalInfoHeight := voteExtensionHeight

	currentBlockInfo, err := q.k.stakingKeeper.GetHistoricalInfo(ctx, stateRootHistoricalInfoHeight)
	if err != nil {
		return nil, err
	}

	nextBlockInfo, err := q.k.stakingKeeper.GetHistoricalInfo(ctx, targetValidatorSetHistoricalInfoHeight)
	if err != nil {
		return nil, err
	}

	nextValidatorSetHash, err := q.calculateValidatorSetRoot(ctx, nextBlockInfo.Valset)
	if err != nil {
		return nil, err
	}

	return &types.VoteExtBody{
		NextValidatorSetHash: nextValidatorSetHash,
		CurrentStateRoot:     currentBlockInfo.Header.AppHash,
		CurrentBlockHeight:   signedStateHeight,
		ActionsReducedRoot:   ActionsReducedRoot,
	}, nil
}

func (q queryServer) calculateValidatorSetRoot(ctx context.Context, validatorSet []stakingtypes.Validator) ([]byte, error) {

	poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)
	merkleRoot := poseidonHash.Hash([]*big.Int{big.NewInt(0)})

	for _, validator := range validatorSet {
		cosmosValidatorPubKey, err := validator.ConsPubKey()
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to read validator consensus public key")
		}

		exists, err := q.k.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, cosmosValidatorPubKey.Bytes())
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to check validator Mina key")
		}
		if !exists {
			return nil, status.Errorf(codes.NotFound, "validator Mina key not found for consensus public key %X", cosmosValidatorPubKey.Bytes())
		}

		minaPubKey, err := q.k.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, cosmosValidatorPubKey.Bytes())
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to load validator Mina key")
		}

		var minaPublicKey keys.PublicKey
		err = minaPublicKey.Unmarshal(minaPubKey)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to decode validator Mina public key")
		}

		input := []*big.Int{minaPublicKey.X}
		if minaPublicKey.IsOdd {
			input = append(input, big.NewInt(1))
		} else {
			input = append(input, big.NewInt(0))
		}
		input = append(input, big.NewInt(validator.ConsensusPower(sdk.DefaultPowerReduction)))

		hashOfValidator := poseidonHash.Hash(input)
		merkleRoot = poseidonHash.Hash([]*big.Int{merkleRoot, hashOfValidator})
	}

	if merkleRoot == nil {
		return nil, status.Error(codes.Internal, "failed to calculate next validator set hash")
	}

	return merkleRoot.Bytes(), nil
}
