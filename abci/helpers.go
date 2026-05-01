package vote_ext

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

func MockSign(voteExtBody VoteExtensionBody) []byte {
	return []byte{}
}

func MockSignatureVerify(signature []byte, message VoteExtensionBody, minaKey []byte, reducedRoot string) bool {
	return true
}

func extractPayload(txs [][]byte) (Payload, error) {

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

func (h *AbciHandler) verifyVoteExtension(ctx context.Context, pl Payload, body VoteExtensionBody) error {

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
func (h *AbciHandler) getValidatorSet(ctx sdk.Context, currentBlockHeight int64) ([]validatorInfo, error) {

	var valInfo []validatorInfo
	var iterErr error

	if ctx.BlockHeight() == currentBlockHeight {
		err := h.stakingKeeper.IterateLastValidators(ctx, func(index int64, validator stakingTypes.ValidatorI) (stop bool) {
			consAddr, err := validator.GetConsAddr()
			if err != nil {
				iterErr = err
				return true
			}

			valInfo = append(valInfo, validatorInfo{
				ConsensusAddr: consAddr,
				Power:         validator.GetConsensusPower(sdk.DefaultPowerReduction),
			})

			return false
		})

		if err != nil {
			return nil, err
		}
		if iterErr != nil {
			return nil, iterErr
		}

		return valInfo, nil
	}

	historicalData, err := h.stakingKeeper.GetHistoricalInfo(ctx, currentBlockHeight)
	if err != nil {
		return nil, err
	}

	validatorSet := historicalData.Valset

	for _, validator := range validatorSet {

		consAddr, err := validator.GetConsAddr()
		if err != nil {
			return nil, err
		}

		consPower := validator.ConsensusPower(sdk.DefaultPowerReduction)

		valInfo = append(valInfo, validatorInfo{
			ConsensusAddr: consAddr,
			Power:         consPower,
		})
	}
	return valInfo, nil
}

// TODO: Move this helper to mina-signer-go
func (h *AbciHandler) calculateValidatorSetRoot(ctx sdk.Context, valInfo []validatorInfo, poseidonHash *poseidon.Poseidon) (*big.Int, error) {

	input := []*big.Int{big.NewInt(0)}
	merkleRoot := poseidonHash.Hash(input)

	for _, validator := range valInfo {
		input = []*big.Int{}

		cosmosValidatorInfo, err := h.stakingKeeper.GetValidatorByConsAddr(ctx, validator.ConsensusAddr)
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
		power := new(big.Int).SetInt64(validator.Power)
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

	valInfoMap := make(map[string]validatorInfo)

	currentValidatorSet, err := h.getValidatorSet(ctx, blockHeight-2)
	if err != nil {
		return false, err
	}

	// Require at least 2/3 signed power to prevent proposer-side signature withholding.
	for _, val := range currentValidatorSet {

		cosmosValidatorPubKey, err := h.getValidatorPublicKey(ctx, val.ConsensusAddr)
		if err != nil {
			return false, err
		}

		valInfoMap[hex.EncodeToString(cosmosValidatorPubKey)] = val
		currentValidatorStakePower += val.Power
	}

	for addr := range pl.Votes {
		validatorInfo, ok := valInfoMap[addr]
		if ok {
			signedStakePower += validatorInfo.Power
		}
	}
	if signedStakePower*3 < currentValidatorStakePower*2 {
		return false, nil
	}

	return true, nil
}

func (h *AbciHandler) constructVoteExtBody(ctx sdk.Context, blockHeight int64) (VoteExtensionBody, error) {

	nextValidatorSet, err := h.getValidatorSet(ctx, blockHeight)
	if err != nil {
		return VoteExtensionBody{}, err
	}

	currentBlockInfo, err := h.stakingKeeper.GetHistoricalInfo(ctx, blockHeight-1)
	if err != nil {
		return VoteExtensionBody{}, err
	}

	poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

	nextValidatorSetHash, err := h.calculateValidatorSetRoot(ctx, nextValidatorSet, poseidonHash)
	if err != nil {
		return VoteExtensionBody{}, err
	}
	if nextValidatorSetHash == nil {
		return VoteExtensionBody{}, err
	}

	return VoteExtensionBody{
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
		currentValidatorSetMap[string(currentValidator.ConsensusAddr)] = true
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
