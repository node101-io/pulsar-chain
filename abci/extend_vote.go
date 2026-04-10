package vote_ext

import (
	"math/big"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
)

const ActionsReducedRoot string = "pulsar"

type validatorInfo struct {
	ConsensusAddr []byte
	Power         int64
}

func MockSign(voteExtBody VoteExtensionBody) []byte {
	return []byte{}
}

func MockSignatureVerify(voteExtBody VoteExtensionBody, minaKey []byte, reducedRoot string) bool {
	return true
}

func (h *AbciHandler) getValidatorSet(ctx sdk.Context, currentBlockHeight int64) ([]validatorInfo, error) {

	historicalData, err := h.stakingKeeper.GetHistoricalInfo(ctx, currentBlockHeight)
	if err != nil {
		return nil, err
	}

	validatorSet := historicalData.Valset

	var valInfo []validatorInfo

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

func (h *AbciHandler) calculateValidatorSetRoot(ctx sdk.Context, valInfo []validatorInfo, poseidonHash *poseidon.Poseidon) (*big.Int, error) {

	input := []*big.Int{big.NewInt(0)}
	merkleRoot := poseidonHash.Hash(input)

	for _, validator := range valInfo {
		input = []*big.Int{}

		minaPubKeyExists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, validator.ConsensusAddr)
		if err != nil {
			return nil, err
		}
		if !minaPubKeyExists {
			return nil, err
		}

		minaPubKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, validator.ConsensusAddr)
		if err != nil {
			return nil, err
		}

		MinaPublicKey, err := keys.PublicKey{}.FromAddress(string(minaPubKey))
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

func (h *AbciHandler) ExtendVoteHandler() sdk.ExtendVoteHandler {
	return func(ctx sdk.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
		if req.GetHeight() < 3 {
			return &abci.ResponseExtendVote{VoteExtension: []byte{}}, nil
		}

		twoBlocksBefore, err := stakingkeeper.Keeper.GetHistoricalInfo(h.stakingKeeper, ctx, req.GetHeight()-2)
		if err != nil {
			return &abci.ResponseExtendVote{VoteExtension: []byte{}}, err
		}

		nextBlockHeight := req.GetHeight() - 1

		poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

		nextValSet, err := h.getValidatorSet(ctx, nextBlockHeight)
		if err != nil {
			return nil, err
		}
		nextValidatorSetHash, err := h.calculateValidatorSetRoot(ctx, nextValSet, poseidonHash)
		if err != nil {
			return nil, err
		}

		body := VoteExtensionBody{
			NextBlockHeight:      nextBlockHeight,
			CurrentStateRoot:     twoBlocksBefore.Header.AppHash,
			NextValidatorSetHash: nextValidatorSetHash.Bytes(),
		}

		bz := MockSign(body)

		return &abci.ResponseExtendVote{VoteExtension: bz}, nil
	}
}
