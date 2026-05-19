package vote_ext

import (
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const ActionsReducedRoot string = "pulsar"

const VoteExtMarker string = "VOTEEXT:"

const AdditionalVoteExtHeight int64 = 3

const NetworkID string = "testnet"

var (
	ErrUnableToReadConsensusParams   error = errors.New("unable to read consensus params")
	ErrInvalidPayload                error = errors.New("invalid payload")
	ErrNotEnoughStakePower           error = errors.New("not enough stake power signed the vote extension")
	ErrMissingSecondaryKey           error = errors.New("missing secondary key")
	ErrInvalidSecondaryKey           error = errors.New("invalid secondary key")
	ErrMissingStakingKeeper          error = errors.New("missing staking keeper")
	ErrMissingKeyregistryKeeper      error = errors.New("missing keyregistry keeper")
	ErrMissingVotePersistenceKeeper  error = errors.New("missing vote persistence keeper")
	ErrVoteExtBodyHashFailed         error = errors.New("failed to hash vote extension body")
	ErrVoteExtSigningFailed          error = errors.New("failed to sign vote extension body")
	ErrVoteExtSignatureMarshalFailed error = errors.New("failed to marshal vote extension signature")
)

func shouldExtendVoteAtHeight(ctx sdk.Context, height int64) (bool, error) {
	cp := ctx.ConsensusParams()
	if cp.Abci == nil {
		return false, ErrUnableToReadConsensusParams
	}

	return height >= cp.Abci.VoteExtensionsEnableHeight+AdditionalVoteExtHeight, nil
}

func shouldRequireProposalPayloadAtHeight(ctx sdk.Context, height int64) (bool, error) {
	cp := ctx.ConsensusParams()
	if cp.Abci == nil {
		return false, ErrUnableToReadConsensusParams
	}

	return height >= cp.Abci.VoteExtensionsEnableHeight+AdditionalVoteExtHeight+1, nil
}
