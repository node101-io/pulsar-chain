package vote_ext

import (
	"bytes"
	"math/big"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	votepersistenceTypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

// Use if you need the validator set of block < N where N is the current block number.
func (h *ABCIHandler) getValidatorSet(ctx sdk.Context, currentBlockHeight int64) ([]stakingTypes.ValidatorI, error) {

	var valInfo []stakingTypes.ValidatorI

	if ctx.BlockHeight() == currentBlockHeight {
		err := h.stakingKeeper.IterateLastValidators(ctx, func(index int64, validator stakingTypes.ValidatorI) (stop bool) {
			valInfo = append(valInfo, validator)
			return false
		})

		if err != nil {
			return nil, err
		}
		sortValidatorsByPower(valInfo)

		return valInfo, nil
	}

	historicalData, err := h.stakingKeeper.GetHistoricalInfo(ctx, currentBlockHeight)
	if err != nil {
		return nil, err
	}
	for _, validator := range historicalData.Valset {
		valInfo = append(valInfo, validator)
	}

	sortValidatorsByPower(valInfo)

	return valInfo, nil
}

func sortValidatorsByPower(validators []stakingTypes.ValidatorI) {
	sort.SliceStable(validators, func(i, j int) bool {
		leftPower := validators[i].GetConsensusPower(sdk.DefaultPowerReduction)
		rightPower := validators[j].GetConsensusPower(sdk.DefaultPowerReduction)

		if leftPower == rightPower {
			leftAddr, err := validators[i].GetConsAddr()
			if err != nil {
				return false
			}
			rightAddr, err := validators[j].GetConsAddr()
			if err != nil {
				return false
			}

			return bytes.Compare(leftAddr, rightAddr) == -1
		}

		return leftPower > rightPower
	})
}

// TODO: Move this helper to mina-signer-go
func (h *ABCIHandler) calculateValidatorSetRoot(ctx sdk.Context, valInfo []stakingTypes.ValidatorI, poseidonHash *poseidon.Poseidon) (*big.Int, error) {

	input := []*big.Int{big.NewInt(0)}
	merkleRoot := poseidonHash.Hash(input)

	for _, validator := range valInfo {
		input = []*big.Int{}

		consAddr, err := validator.GetConsAddr()
		if err != nil {
			continue
		}

		cosmosValidatorInfo, err := h.stakingKeeper.GetValidatorByConsAddr(ctx, sdk.ConsAddress(consAddr))
		if err != nil {
			return nil, err
		}

		cosmosValidatorPubKey, err := cosmosValidatorInfo.ConsPubKey()
		if err != nil {
			return nil, err
		}

		minaPubKeyExists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, cosmosValidatorPubKey.Bytes())
		if err != nil {
			return nil, err
		}
		if !minaPubKeyExists {
			return nil, err
		}

		minaPubKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, cosmosValidatorPubKey.Bytes())
		if err != nil {
			return nil, err
		}

		var MinaPublicKey keys.PublicKey
		err = MinaPublicKey.Unmarshal(minaPubKey)
		if err != nil {
			return nil, err
		}

		input = append(input, MinaPublicKey.X)
		if MinaPublicKey.IsOdd {
			input = append(input, big.NewInt(1))
		} else {
			input = append(input, big.NewInt(0))
		}
		power := new(big.Int).SetInt64(validator.GetConsensusPower(sdk.DefaultPowerReduction))
		input = append(input, power)

		hashOfAddr := poseidonHash.Hash(input)

		input = []*big.Int{merkleRoot, hashOfAddr}

		merkleRoot = poseidonHash.Hash(input)
	}

	return merkleRoot, nil

}

func (h *ABCIHandler) constructVoteExtBody(ctx sdk.Context, blockHeight int64) (votepersistenceTypes.VoteExtBody, error) {

	nextValidatorSet, err := h.getValidatorSet(ctx, blockHeight)
	if err != nil {
		return votepersistenceTypes.VoteExtBody{}, err
	}

	currentBlockInfo, err := h.stakingKeeper.GetHistoricalInfo(ctx, blockHeight-1)
	if err != nil {
		return votepersistenceTypes.VoteExtBody{}, err
	}

	poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

	nextValidatorSetHash, err := h.calculateValidatorSetRoot(ctx, nextValidatorSet, poseidonHash)
	if err != nil {
		return votepersistenceTypes.VoteExtBody{}, err
	}
	if nextValidatorSetHash == nil {
		return votepersistenceTypes.VoteExtBody{}, err
	}

	return votepersistenceTypes.VoteExtBody{
		NextValidatorSetHash: nextValidatorSetHash.Bytes(),
		CurrentStateRoot:     currentBlockInfo.Header.AppHash,
		CurrentBlockHeight:   blockHeight - 1,
		ActionsReducedRoot:   ActionsReducedRoot,
	}, nil
}

func (h *ABCIHandler) getValidatorPublicKey(ctx sdk.Context, validatorAddr []byte) ([]byte, error) {

	cosmosValidatorInfo, err := h.stakingKeeper.GetValidatorByConsAddr(ctx, sdk.ConsAddress(validatorAddr))
	if err != nil {
		return nil, err
	}
	cosmosValidatorPubKey, err := cosmosValidatorInfo.ConsPubKey()
	if err != nil {
		return nil, err
	}

	return cosmosValidatorPubKey.Bytes(), nil
}
