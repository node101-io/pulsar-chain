package vote_ext

import "errors"

const ActionsReducedRoot string = "pulsar"

const VoteExtMarker string = "VOTEEXT:"

const AdditionalVoteExtHeight int64 = 3

const NetworkID string = "testnet"

var ErrUnableToReadConsensusParams error = errors.New("unable to read consensus params")
