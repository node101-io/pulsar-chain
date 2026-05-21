package abci

import (
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TODO: Will remove this when we add Bridge module
const ActionsReducedRoot string = "pulsar"

// VoteExtMarker reserves the first proposal transaction slot for ABCI vote-extension payloads.
// Honest proposers prepend exactly one marker-prefixed internal payload before user transactions.
const VoteExtMarker = "PULSAR_ABCI_VOTE_EXT_PAYLOAD:"

var voteExtMarkerBytes = []byte(VoteExtMarker)

const AdditionalVoteExtHeight int64 = 3

const NetworkID string = "testnet"

var (
	ErrUnableToReadConsensusParams     error = errors.New("unable to read consensus params")
	ErrInvalidPayload                  error = errors.New("invalid payload")
	ErrInvalidPayloadHeight            error = errors.New("invalid payload height")
	ErrVoteExtPayloadNotFound          error = errors.New("vote extension payload not found")
	ErrNoVoteExtensionsForPayload      error = errors.New("no vote extensions for payload")
	ErrVoteExtPayloadTooLarge          error = errors.New("vote extension payload exceeds max tx bytes")
	ErrNotEnoughStakePower             error = errors.New("not enough stake power signed the vote extension")
	ErrMissingSecondaryKey             error = errors.New("missing secondary key")
	ErrInvalidSecondaryKey             error = errors.New("invalid secondary key")
	ErrMissingStakingKeeper            error = errors.New("missing staking keeper")
	ErrMissingKeyregistryKeeper        error = errors.New("missing keyregistry keeper")
	ErrMissingVotePersistenceKeeper    error = errors.New("missing vote persistence keeper")
	ErrVoteExtBodyHashFailed           error = errors.New("failed to hash vote extension body")
	ErrVoteExtSigningFailed            error = errors.New("failed to sign vote extension body")
	ErrVoteExtSignatureMarshalFailed   error = errors.New("failed to marshal vote extension signature")
	ErrInvalidVoteExtReducedRoot       error = errors.New("invalid vote extension reduced root")
	ErrInvalidVoteExtMinaPublicKey     error = errors.New("invalid vote extension mina public key")
	ErrInvalidVoteExtSignatureEncoding error = errors.New("invalid vote extension signature encoding")
	ErrInvalidVoteExtSignature         error = errors.New("invalid vote extension signature")
	ErrValidatorMinaKeyNotFound        error = errors.New("validator mina key not found")
	ErrValidatorSetRootHashFailed      error = errors.New("failed to hash validator set root")
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
