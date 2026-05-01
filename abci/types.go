package vote_ext

const ActionsReducedRoot string = "pulsar"

var VoteExtMarker string = "VOTEEXT:"

type Payload struct {
	Height int64             `json:"height"`
	Votes  map[string][]byte `json:"votes"`
}
