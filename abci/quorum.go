package abci

import (
	"errors"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/poseidon"
	keyregistryTypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
	votepersistenceTypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

type validatorVoteInfo struct {
	power int64
}

type consPubKeyKey string

type verifiedPayloadVoteExtension struct {
	consensusPublicKey []byte
	minaPublicKey      []byte
	voteExtension      []byte
	power              int64
}

type verifiedPayloadVoteExtensions struct {
	votes       []verifiedPayloadVoteExtension
	signedPower int64
	totalPower  int64
}

func consPubKeyMapKey(consPubKey []byte) consPubKeyKey {
	return consPubKeyKey(consPubKey)
}

func (h *ABCIHandler) buildValidatorVoteIndex(ctx sdk.Context, voteExtensionHeight int64) (map[consPubKeyKey]validatorVoteInfo, int64, error) {
	validatorVoteIndex := make(map[consPubKeyKey]validatorVoteInfo)
	var totalPower int64

	// Quorum is measured against the validator set that was active for the
	// consensus height that produced these vote extensions.
	currentValidatorSet, err := h.getValidatorSet(ctx, voteExtensionHeight)
	if err != nil {
		return nil, 0, err
	}

	for _, val := range currentValidatorSet {
		consAddr, err := val.GetConsAddr()
		if err != nil {
			return nil, 0, err
		}

		cosmosValidatorPubKey, err := h.getConsPubKeyByConsAddr(ctx, consAddr)
		if err != nil {
			return nil, 0, err
		}

		power := val.GetConsensusPower(sdk.DefaultPowerReduction)
		validatorVoteIndex[consPubKeyMapKey(cosmosValidatorPubKey)] = validatorVoteInfo{
			power: power,
		}
		totalPower += power
	}

	return validatorVoteIndex, totalPower, nil
}

func (h *ABCIHandler) validatePayloadVoteExtensions(ctx sdk.Context, voteExtensionHeight int64, pl Payload, body votepersistenceTypes.VoteExtBody) (verifiedPayloadVoteExtensions, error) {
	// voteExtensionHeight is the consensus height of the vote extensions carried
	// by the payload. The body may refer to an older signed state height, but the
	// eligible validators and quorum power are still determined at this height.
	validatorVoteIndex, totalPower, err := h.buildValidatorVoteIndex(ctx, voteExtensionHeight)
	if err != nil {
		return verifiedPayloadVoteExtensions{}, err
	}

	poseidonHash := poseidon.NewPoseidon()
	seenConsensusPubKeys := make(map[consPubKeyKey]struct{})
	verifiedVotes := verifiedPayloadVoteExtensions{totalPower: totalPower}

	for _, vote := range pl.VoteExtensions {
		consPubKeyKey := consPubKeyMapKey(vote.ConsensusPublicKey)
		validatorInfo, ok := validatorVoteIndex[consPubKeyKey]
		if !ok {
			continue
		}

		if _, seen := seenConsensusPubKeys[consPubKeyKey]; seen {
			continue
		}

		seenConsensusPubKeys[consPubKeyKey] = struct{}{}

		exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, vote.ConsensusPublicKey)
		if err != nil {
			return verifiedPayloadVoteExtensions{}, err
		}
		if !exists {
			return verifiedPayloadVoteExtensions{}, keyregistryTypes.ErrValidatorNotRegistered
		}

		minaKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, vote.ConsensusPublicKey)
		if err != nil {
			return verifiedPayloadVoteExtensions{}, err
		}

		if err := verifyVoteExtSig(poseidonHash, vote.VoteExtension, body, minaKey, ActionsReducedRoot); err != nil {
			if errors.Is(err, ErrInvalidVoteExtSignatureEncoding) || errors.Is(err, ErrInvalidVoteExtSignature) {
				return verifiedPayloadVoteExtensions{}, votepersistenceTypes.ErrInvalidVoteExtension.Wrap(err.Error())
			}

			return verifiedPayloadVoteExtensions{}, err
		}

		verifiedVotes.votes = append(verifiedVotes.votes, verifiedPayloadVoteExtension{
			consensusPublicKey: vote.ConsensusPublicKey,
			minaPublicKey:      minaKey,
			voteExtension:      vote.VoteExtension,
			power:              validatorInfo.power,
		})
		verifiedVotes.signedPower += validatorInfo.power

	}

	return verifiedVotes, nil
}

func hasAtLeastTwoThirdsPower(signedPower, totalPower int64) bool {
	if totalPower <= 0 {
		return false
	}

	signed := big.NewInt(signedPower)
	signed.Mul(signed, big.NewInt(3))

	total := big.NewInt(totalPower)
	total.Mul(total, big.NewInt(2))

	return signed.Cmp(total) >= 0
}
