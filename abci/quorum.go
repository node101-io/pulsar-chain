package vote_ext

import (
	"encoding/hex"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	votepersistenceTypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

func (h *AbciHandler) checkStakePower(ctx sdk.Context, blockHeight int64, pl Payload, body votepersistenceTypes.VoteExtBody) (bool, error) {
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

		cosmosValidatorPubKey, err := h.getValidatorPublicKey(ctx, consAddr)
		if err != nil {
			return false, err
		}

		valInfoMap[hex.EncodeToString(cosmosValidatorPubKey)] = val
		currentValidatorStakePower += val.GetConsensusPower(sdk.DefaultPowerReduction)
	}

	for _, vote := range pl.Votes {

		validatorInfo, ok := valInfoMap[vote.ConsensusPublicKey]
		if !ok {
			continue
		}

		pk, err := hex.DecodeString(vote.ConsensusPublicKey)
		if err != nil {
			return false, err
		}

		if validatorSeen[vote.ConsensusPublicKey] {
			continue
		}

		validatorSeen[vote.ConsensusPublicKey] = true

		exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, pk)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, types.ErrValidatorNotRegistered
		}

		minaKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, pk)
		if err != nil {
			return false, err
		}

		poseidon := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

		if !verifyVoteExtSig(poseidon, vote.VoteExtension, body, minaKey, ActionsReducedRoot) {
			return false, votepersistenceTypes.ErrInvalidVoteExtension.Wrap("invalid signature")
		}

		signedStakePower += validatorInfo.GetConsensusPower(sdk.DefaultPowerReduction)

	}
	if signedStakePower*3 < currentValidatorStakePower*2 {
		return false, nil
	}

	return true, nil
}
