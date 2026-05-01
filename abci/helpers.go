package vote_ext

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/pulsar-chain/x/votepersistence/types"
	votepersistenceTypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

func MockSign(voteExtBody votepersistenceTypes.VoteExtBody) []byte {
	return []byte{}
}

func MockSignatureVerify(signature []byte, message votepersistenceTypes.VoteExtBody, minaKey []byte, reducedRoot string) bool {
	return true
}

func extractPayload(txs [][]byte) (Payload, error) {

	if len(txs) == 0 {
		return Payload{}, ErrEmptyBlock
	}

	voteExtensionTx := txs[0]

	if !strings.Contains(string(txs[0]), VoteExtMarker) {
		return Payload{}, types.ErrVoteExtMarkerNotFound
	}

	voteExtensionTx = voteExtensionTx[len([]byte(VoteExtMarker)):]

	var pl Payload

	err := json.Unmarshal(voteExtensionTx, &pl)
	if err != nil {
		return Payload{}, err
	}
	return pl, nil
}

func (h *AbciHandler) verifyVoteExtension(ctx context.Context, pl Payload, body votepersistenceTypes.VoteExtBody) error {

	for publicKey, vote := range pl.Votes {

		pk, err := hex.DecodeString(publicKey)
		if err != nil {
			return err
		}

		exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, pk)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("")
		}

		minaKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, pk)
		if err != nil {
			return err
		}

		if !MockSignatureVerify(vote, body, minaKey, ActionsReducedRoot) {
			return types.ErrInvalidVoteExtension.Wrap("invalid signature")
		}

	}

	return nil
}

// Use if you need the validator set of block < N where N is the current block number.
func (h *AbciHandler) getValidatorSet(ctx sdk.Context, currentBlockHeight int64) ([]stakingTypes.ValidatorI, error) {

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
func (h *AbciHandler) calculateValidatorSetRoot(ctx sdk.Context, valInfo []stakingTypes.ValidatorI, poseidonHash *poseidon.Poseidon) (*big.Int, error) {

	input := []*big.Int{big.NewInt(0)}
	merkleRoot := poseidonHash.Hash(input)

	for _, validator := range valInfo {
		input = []*big.Int{}

		consAddr, err := validator.GetConsAddr()
		if err != nil {
			continue
		}

		cosmosValidatorInfo, err := h.stakingKeeper.GetValidatorByConsAddr(ctx, consAddr)
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

func (h *AbciHandler) checkStakePower(ctx sdk.Context, blockHeight int64, pl Payload) (bool, error) {
	var signedStakePower int64
	var currentValidatorStakePower int64

	valInfoMap := make(map[string]stakingTypes.ValidatorI)

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

	for addr := range pl.Votes {
		validatorInfo, ok := valInfoMap[addr]
		if ok {
			signedStakePower += validatorInfo.GetConsensusPower(sdk.DefaultPowerReduction)
		}
	}
	if signedStakePower*3 < currentValidatorStakePower*2 {
		return false, nil
	}

	return true, nil
}

func (h *AbciHandler) constructVoteExtBody(ctx sdk.Context, blockHeight int64) (votepersistenceTypes.VoteExtBody, error) {

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

func (h *AbciHandler) getValidatorPublicKey(ctx sdk.Context, validatorAddr []byte) ([]byte, error) {

	cosmosValidatorInfo, err := h.stakingKeeper.GetValidatorByConsAddr(ctx, validatorAddr)
	if err != nil {
		return nil, err
	}
	cosmosValidatorPubKey, err := cosmosValidatorInfo.ConsPubKey()
	if err != nil {
		return nil, err
	}

	return cosmosValidatorPubKey.Bytes(), nil
}

func (h *AbciHandler) constructPayload(ctx sdk.Context, blockHeight int64, voteExtensions []abci.ExtendedVoteInfo) (Payload, error) {
	voteExtsForGivenBlock := make(map[string][]byte)
	currentValidatorSetMap := make(map[string]bool)

	currentValidatorSet, err := h.getValidatorSet(ctx, blockHeight-2)
	if err != nil {
		return Payload{}, err
	}

	for _, currentValidator := range currentValidatorSet {

		consAddr, err := currentValidator.GetConsAddr()
		if err != nil {
			continue
		}

		currentValidatorSetMap[string(consAddr)] = true
	}

	for i, vote := range voteExtensions {

		if !currentValidatorSetMap[string(vote.Validator.Address)] {
			continue
		}

		cosmosValidatorPubKey, err := h.getValidatorPublicKey(ctx, vote.Validator.Address)
		if err != nil {
			return Payload{}, err
		}

		voteExtsForGivenBlock[hex.EncodeToString(cosmosValidatorPubKey)] = voteExtensions[i].VoteExtension
	}

	if len(voteExtsForGivenBlock) == 0 {
		return Payload{}, nil
	}

	return Payload{Height: blockHeight - 1, Votes: voteExtsForGivenBlock}, nil
}
