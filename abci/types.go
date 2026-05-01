package vote_ext

import "errors"

const ActionsReducedRoot string = "pulsar"

var VoteExtMarker string = "VOTEEXT:"

var ErrNotEnoughStakePower error = errors.New("not enough stake power")
