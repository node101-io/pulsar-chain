package vote_ext

import "errors"

const ActionsReducedRoot string = "pulsar"

var VoteExtMarker string = "VOTEEXT:"

var ErrEmptyBlock error = errors.New("this block has no transaction")

type Payload struct {
	Height int64             `json:"height"`
	Votes  map[string][]byte `json:"votes"`
}
