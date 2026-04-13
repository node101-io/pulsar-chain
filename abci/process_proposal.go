package vote_ext

import (
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
)

func (h *AbciHandler) ProcessProposalHandler() sdk.ProcessProposalHandler {

	return func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {

		// If height is 1, we won't have any votes thus skip the proposal
		if req.GetHeight() == 1 {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
		}

		// vote ext reconstruct
		currentState, err := stakingkeeper.Keeper.GetHistoricalInfo(h.stakingKeeper, ctx, req.GetHeight()-2)
		if err != nil {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
		}

		valInfo, err := h.getValidatorSet(ctx, req.GetHeight()-1)
		if err != nil {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
		}

		poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

		nextValidatorSetHash, err := h.calculateValidatorSetRoot(ctx, valInfo, poseidonHash)
		if err != nil {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
		}
		if nextValidatorSetHash == nil {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
		}

		body := VoteExtensionBody{
			NextBlockHeight:      req.GetHeight() - 1,
			CurrentStateRoot:     currentState.Header.AppHash,
			NextValidatorSetHash: nextValidatorSetHash.Bytes(),
		}

		var signedStakePower int64
		var currentValidatorStakePower int64
		voteExtMap := h.fetchVotes(uint64(req.GetHeight()) - 2)

		for _, val := range valInfo {
			if voteExtMap[string(val.ConsensusAddr)] != nil {
				signedStakePower += val.Power
			}
			currentValidatorStakePower += val.Power
		}

		if float64(signedStakePower) < float64(currentValidatorStakePower)*AcceptanceRatio {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
		}

		MockSignatureVerify(body, h.secondaryKey.PublicKey.X.Bytes(), ActionsReducedRoot)

		// Vote extension successfully verified
		return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
	}

}
