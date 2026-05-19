package abci

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	votepersistenceTypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

func (h *ABCIHandler) checkStakePower(ctx sdk.Context, blockHeight int64, pl Payload, body votepersistenceTypes.VoteExtBody) (bool, error) {
	var signedStakePower int64
	var currentValidatorStakePower int64

	valInfoMap := make(map[string]stakingTypes.ValidatorI)
	validatorSeen := make(map[string]bool)

	currentValidatorSet, err := h.getValidatorSet(ctx, blockHeight-2)
	if err != nil {
		return false, err
	}

	// Require at least 2/3 signed power to prevent proposer-side signature withholding.
	for _, val := range currentValidatorSet {

		consAddr, err := val.GetConsAddr()
		if err != nil {
			continue
		}

		cosmosValidatorPubKey, err := h.getConsPubKeyByConsAddr(ctx, consAddr)
		if err != nil {
			return false, err
		}

		valInfoMap[string(cosmosValidatorPubKey)] = val
		currentValidatorStakePower += val.GetConsensusPower(sdk.DefaultPowerReduction)
	}

	for _, vote := range pl.Votes {

		validatorInfo, ok := valInfoMap[string(vote.ConsensusPublicKey)]
		if !ok {
			continue
		}

		if validatorSeen[string(vote.ConsensusPublicKey)] {
			continue
		}

		validatorSeen[string(vote.ConsensusPublicKey)] = true

		exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, vote.ConsensusPublicKey)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, types.ErrValidatorNotRegistered
		}

		minaKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, vote.ConsensusPublicKey)
		if err != nil {
			return false, err
		}

		poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

		if err := verifyVoteExtSig(poseidonHash, vote.VoteExtension, body, minaKey, ActionsReducedRoot); err != nil {
			return false, votepersistenceTypes.ErrInvalidVoteExtension.Wrap(err.Error())
		}

		signedStakePower += validatorInfo.GetConsensusPower(sdk.DefaultPowerReduction)

	}
	if signedStakePower*3 < currentValidatorStakePower*2 {
		return false, nil
	}

	return true, nil
}
