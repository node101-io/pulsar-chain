package abci

import (
	"bytes"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/merklelist"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/mina-signer-go/publickey"
	votepersistenceTypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

// getValidatorSet returns the validator set for validatorSetHeight. The current
// block height is read from LastValidators; earlier heights are read from
// staking historical info.
func (h *ABCIHandler) getValidatorSet(ctx sdk.Context, validatorSetHeight int64) ([]stakingTypes.ValidatorI, error) {

	var valInfo []stakingTypes.ValidatorI

	if ctx.BlockHeight() == validatorSetHeight {
		err := h.stakingKeeper.IterateLastValidators(ctx, func(index int64, validator stakingTypes.ValidatorI) (stop bool) {
			valInfo = append(valInfo, validator)
			return false
		})

		if err != nil {
			return nil, err
		}
		if err := sortValidatorsByPower(valInfo); err != nil {
			return nil, err
		}

		return valInfo, nil
	}

	historicalData, err := h.stakingKeeper.GetHistoricalInfo(ctx, validatorSetHeight)
	if err != nil {
		return nil, err
	}
	for _, validator := range historicalData.Valset {
		valInfo = append(valInfo, validator)
	}

	if err := sortValidatorsByPower(valInfo); err != nil {
		return nil, err
	}

	return valInfo, nil
}

func sortValidatorsByPower(validators []stakingTypes.ValidatorI) error {
	type validatorSortEntry struct {
		validator        stakingTypes.ValidatorI
		consensusAddress []byte
		consensusPower   int64
	}

	entries := make([]validatorSortEntry, 0, len(validators))
	for _, validator := range validators {
		consAddr, err := validator.GetConsAddr()
		if err != nil {
			return fmt.Errorf("failed to read validator consensus address: %w", err)
		}

		entries = append(entries, validatorSortEntry{
			validator:        validator,
			consensusAddress: consAddr,
			consensusPower:   validator.GetConsensusPower(sdk.DefaultPowerReduction),
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].consensusPower == entries[j].consensusPower {
			return bytes.Compare(entries[i].consensusAddress, entries[j].consensusAddress) < 0
		}

		return entries[i].consensusPower > entries[j].consensusPower
	})

	for i, entry := range entries {
		validators[i] = entry.validator
	}

	return nil
}

func (h *ABCIHandler) calculateValidatorSetRoot(ctx sdk.Context, valInfo []stakingTypes.ValidatorI, poseidonHash *poseidon.Poseidon) ([]byte, error) {
	if poseidonHash == nil {
		return nil, ErrValidatorSetRootHashFailed
	}

	validatorRoot, err := merklelist.NewMerkleList(ValidatorSetMerklePrefix)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidatorSetRootHashFailed, err)
	}

	for _, validator := range valInfo {
		consAddr, err := validator.GetConsAddr()
		if err != nil {
			return nil, fmt.Errorf("failed to read validator consensus address: %w", err)
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
			return nil, fmt.Errorf("%w: consensus public key %X", ErrValidatorMinaKeyNotFound, cosmosValidatorPubKey.Bytes())
		}

		minaPubKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, cosmosValidatorPubKey.Bytes())
		if err != nil {
			return nil, err
		}

		if _, err := publickey.NewPublicKeyFromBytes(minaPubKey, h.networkID); err != nil {
			return nil, err
		}

		entryBytes, err := encodeValidatorSetEntryForHash(minaPubKey, validator.GetConsensusPower(sdk.DefaultPowerReduction))
		if err != nil {
			return nil, err
		}
		entryHash, err := poseidonHash.HashWithPrefix(ValidatorSetEntryHashPrefix, entryBytes)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrValidatorSetRootHashFailed, err)
		}
		if err := validatorRoot.Append(entryHash); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrValidatorSetRootHashFailed, err)
		}
	}

	return validatorRoot.Root(), nil
}

func (h *ABCIHandler) constructVoteExtBody(ctx sdk.Context, voteExtensionHeight int64) (votepersistenceTypes.VoteExtBody, error) {
	// ExtendVote(N) signs the transition from state N-2 to state N-1. The body
	// stores the source/current state height, while the validator-set root commits
	// to the target validator set available after N-1.
	signedStateHeight := voteExtensionHeight - 2
	// Staking stores HistoricalInfo(H).Header.AppHash as the app hash entering
	// height H, which is the state root after H-1. Reading H=N-1 gives state N-2.
	stateRootHistoricalInfoHeight := voteExtensionHeight - 1
	// At ExtendVote(N), the committed staking state already contains the validator
	// set after N-1; that is the target validator set for the signed transition.
	targetValidatorSetHeight := voteExtensionHeight

	nextValidatorSet, err := h.getValidatorSet(ctx, targetValidatorSetHeight)
	if err != nil {
		return votepersistenceTypes.VoteExtBody{}, err
	}

	currentBlockInfo, err := h.stakingKeeper.GetHistoricalInfo(ctx, stateRootHistoricalInfoHeight)
	if err != nil {
		return votepersistenceTypes.VoteExtBody{}, err
	}

	poseidonHash := poseidon.NewPoseidon()

	nextValidatorSetRoot, err := h.calculateValidatorSetRoot(ctx, nextValidatorSet, poseidonHash)
	if err != nil {
		return votepersistenceTypes.VoteExtBody{}, err
	}
	if nextValidatorSetRoot == nil {
		return votepersistenceTypes.VoteExtBody{}, ErrValidatorSetRootHashFailed
	}

	return votepersistenceTypes.VoteExtBody{
		NextValidatorSetHash: nextValidatorSetRoot,
		CurrentStateRoot:     currentBlockInfo.Header.AppHash,
		CurrentBlockHeight:   signedStateHeight,
		ActionsReducedRoot:   ActionsReducedRoot,
	}, nil
}

func (h *ABCIHandler) getConsPubKeyByConsAddr(ctx sdk.Context, validatorAddr []byte) ([]byte, error) {

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
