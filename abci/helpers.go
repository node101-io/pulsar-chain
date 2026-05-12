package vote_ext

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"sort"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	minasignature "github.com/node101-io/mina-signer-go/signature"
	abcipb "github.com/node101-io/pulsar-chain/api/pulsarchain/abci"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
	votepersistenceTypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

func (s *SecondaryKey) SignVoteExtBody(poseidon *poseidon.Poseidon, voteExtBody votepersistenceTypes.VoteExtBody) []byte {
	if s == nil || s.SecretKey == nil {
		return nil
	}

	innerHash := poseidon.Hash([]*big.Int{
		new(big.Int).SetBytes(voteExtBody.NextValidatorSetHash),
		new(big.Int).SetBytes(voteExtBody.CurrentStateRoot),
		big.NewInt(voteExtBody.CurrentBlockHeight),
	})
	if innerHash == nil {
		return nil
	}

	msgHash := poseidon.Hash([]*big.Int{
		innerHash,
		new(big.Int).SetBytes([]byte(voteExtBody.ActionsReducedRoot)),
	})
	if msgHash == nil {
		return nil
	}

	sig, err := s.SecretKey.SignFieldElement(msgHash, NetworkID)
	if err != nil {
		return nil
	}

	bz, err := sig.MarshalBytes()
	if err != nil {
		return nil
	}

	return bz
}

func verifyVoteExtSig(poseidon *poseidon.Poseidon, signature []byte, message votepersistenceTypes.VoteExtBody, minaKey []byte, reducedRoot string) bool {
	if message.ActionsReducedRoot != reducedRoot {
		return false
	}

	var pubKey keys.PublicKey
	if err := pubKey.Unmarshal(minaKey); err != nil {
		return false
	}

	innerHash := poseidon.Hash([]*big.Int{
		new(big.Int).SetBytes(message.NextValidatorSetHash),
		new(big.Int).SetBytes(message.CurrentStateRoot),
		big.NewInt(message.CurrentBlockHeight),
	})
	if innerHash == nil {
		return false
	}

	msgHash := poseidon.Hash([]*big.Int{
		innerHash,
		new(big.Int).SetBytes([]byte(message.ActionsReducedRoot)),
	})
	if msgHash == nil {
		return false
	}

	var sig minasignature.Signature
	if err := sig.UnmarshalBytes(signature); err != nil {
		return false
	}

	return pubKey.VerifyFieldElement(&sig, msgHash, NetworkID)
}

func extractPayload(txs [][]byte) (abcipb.Payload, error) {

	if len(txs) == 0 {
		return abcipb.Payload{}, nil
	}

	if !bytes.HasPrefix(txs[0], []byte(VoteExtMarker)) {
		return abcipb.Payload{}, votepersistenceTypes.ErrVoteExtMarkerNotFound
	}

	voteExtensionTx := txs[0][len([]byte(VoteExtMarker)):]

	var pl abcipb.Payload

	err := pl.Unmarshal(voteExtensionTx)
	if err != nil {
		return abcipb.Payload{}, err
	}
	return pl, nil
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

func (h *AbciHandler) checkStakePower(ctx sdk.Context, blockHeight int64, pl abcipb.Payload, body votepersistenceTypes.VoteExtBody) (bool, error) {
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

func (h *AbciHandler) constructPayload(ctx sdk.Context, blockHeight int64, voteExtensions []abci.ExtendedVoteInfo) (abcipb.Payload, error) {

	var voteExtsForGivenBlock []*abcipb.Votes

	currentValidatorSetMap := make(map[string]bool)

	currentValidatorSet, err := h.getValidatorSet(ctx, blockHeight-2)
	if err != nil {
		return abcipb.Payload{}, err
	}

	for _, currentValidator := range currentValidatorSet {

		consAddr, err := currentValidator.GetConsAddr()
		if err != nil {
			continue
		}

		currentValidatorSetMap[string(consAddr)] = true
	}

	for _, vote := range voteExtensions {

		if !currentValidatorSetMap[string(vote.Validator.Address)] {
			continue
		}

		if vote.BlockIdFlag != tmproto.BlockIDFlagCommit {
			continue
		}

		if len(vote.VoteExtension) == 0 {
			continue
		}

		cosmosValidatorPubKey, err := h.getValidatorPublicKey(ctx, vote.Validator.Address)
		if err != nil {
			return abcipb.Payload{}, err
		}

		voteExtsForGivenBlock = append(voteExtsForGivenBlock, &abcipb.Votes{
			ConsensusPublicKey: hex.EncodeToString(cosmosValidatorPubKey),
			VoteExtension:      vote.VoteExtension,
		})
	}

	if len(voteExtsForGivenBlock) == 0 {
		return abcipb.Payload{}, nil
	}

	return abcipb.Payload{Height: blockHeight - 1, Votes: voteExtsForGivenBlock}, nil
}
