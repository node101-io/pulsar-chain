package abci

import (
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ActionsReducedRoot is a temporary bridge-domain placeholder used in vote-extension
// body signing until the Bridge module provides the real reduced root.
const ActionsReducedRoot string = "pulsar"

// VoteExtMarker reserves the first proposal transaction slot for ABCI vote-extension payloads.
// Honest proposers prepend exactly one marker-prefixed internal payload before user transactions.
const VoteExtMarker = "PULSAR_ABCI_VOTE_EXT_PAYLOAD:"

var voteExtMarkerBytes = []byte(VoteExtMarker)

// MinPulsarVoteExtensionHeight is the first absolute chain height where
// validators can sign a Pulsar transition proof. ExtendVote(N) signs the
// transition from state N-2 to state N-1: the state root for N-2 comes from
// HistoricalInfo(N-1).Header.AppHash, and the target validator set after N-1 is
// available from the committed staking state at height N. Height 2 is therefore
// the first height with both sides of that transition available.
const MinPulsarVoteExtensionHeight int64 = 2

// TODO: Move this into chain/app configuration before non-local testnets. The
// Mina signature domain must match on both signing and verification paths.
const NetworkID string = "testnet"

var (
	// Consensus parameter errors.
	ErrUnableToReadConsensusParams error = errors.New("unable to read consensus params")

	// Payload construction and validation errors.
	ErrInvalidPayload             error = errors.New("invalid payload")
	ErrInvalidPayloadHeight       error = errors.New("invalid payload height")
	ErrVoteExtPayloadNotFound     error = errors.New("vote extension payload not found")
	ErrNoVoteExtensionsForPayload error = errors.New("no vote extensions for payload")
	ErrVoteExtPayloadTooLarge     error = errors.New("vote extension payload exceeds max tx bytes")

	// Quorum validation errors.
	ErrNotEnoughStakePower error = errors.New("not enough stake power signed the vote extension")

	// ABCI handler initialization errors.
	ErrMissingSecondaryKey          error = errors.New("missing secondary key")
	ErrInvalidSecondaryKey          error = errors.New("invalid secondary key")
	ErrMissingStakingKeeper         error = errors.New("missing staking keeper")
	ErrMissingKeyregistryKeeper     error = errors.New("missing keyregistry keeper")
	ErrMissingVotePersistenceKeeper error = errors.New("missing vote persistence keeper")

	// Vote-extension signing and verification errors.
	ErrVoteExtBodyHashFailed           error = errors.New("failed to hash vote extension body")
	ErrVoteExtSigningFailed            error = errors.New("failed to sign vote extension body")
	ErrVoteExtSignatureMarshalFailed   error = errors.New("failed to marshal vote extension signature")
	ErrInvalidVoteExtReducedRoot       error = errors.New("invalid vote extension reduced root")
	ErrInvalidVoteExtMinaPublicKey     error = errors.New("invalid vote extension mina public key")
	ErrInvalidVoteExtSignatureEncoding error = errors.New("invalid vote extension signature encoding")
	ErrInvalidVoteExtSignature         error = errors.New("invalid vote extension signature")

	// Validator-set root construction errors.
	ErrValidatorMinaKeyNotFound   error = errors.New("validator mina key not found")
	ErrValidatorSetRootHashFailed error = errors.New("failed to hash validator set root")
)

func shouldExtendVoteAtHeight(ctx sdk.Context, height int64) (bool, error) {
	cp := ctx.ConsensusParams()
	if cp.Abci == nil {
		return false, ErrUnableToReadConsensusParams
	}

	// VoteExtensionsEnableHeight is a CometBFT consensus height. Pulsar may need
	// to clamp it forward to the first height where its transition proof body can
	// be constructed.
	firstVoteExtensionHeight := firstPulsarVoteExtensionHeight(cp.Abci.VoteExtensionsEnableHeight)
	if firstVoteExtensionHeight == 0 {
		return false, nil
	}

	return height >= firstVoteExtensionHeight, nil
}

func shouldRequireProposalPayloadAtHeight(ctx sdk.Context, height int64) (bool, error) {
	cp := ctx.ConsensusParams()
	if cp.Abci == nil {
		return false, ErrUnableToReadConsensusParams
	}

	// Vote extensions produced at height N are exposed to the proposer at height
	// N+1, so proposal payloads become required one block after the first Pulsar
	// vote-extension height.
	firstVoteExtensionHeight := firstPulsarVoteExtensionHeight(cp.Abci.VoteExtensionsEnableHeight)
	if firstVoteExtensionHeight == 0 {
		return false, nil
	}

	return height >= firstVoteExtensionHeight+1, nil
}

func firstPulsarVoteExtensionHeight(voteExtensionsEnableHeight int64) int64 {
	if voteExtensionsEnableHeight == 0 {
		return 0
	}
	// Heights below MinPulsarVoteExtensionHeight are valid CometBFT enable
	// heights, but Pulsar cannot construct a complete transition body before its
	// own minimum height.
	if voteExtensionsEnableHeight < MinPulsarVoteExtensionHeight {
		return MinPulsarVoteExtensionHeight
	}

	return voteExtensionsEnableHeight
}
