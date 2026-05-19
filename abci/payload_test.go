package abci

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractPayloadReportsMissingPayload(t *testing.T) {
	payload, found, err := extractPayload(nil)
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, payload.Votes)

	payload, found, err = extractPayload([][]byte{[]byte("user-tx")})
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, payload.Votes)
}

func TestExtractPayloadRejectsInvalidPayloadBody(t *testing.T) {
	_, found, err := extractPayload([][]byte{[]byte(VoteExtMarker)})
	require.True(t, found)
	require.ErrorIs(t, err, ErrInvalidPayload)

	malformedPayload := append([]byte(VoteExtMarker), []byte{0xff, 0xff, 0xff}...)
	_, found, err = extractPayload([][]byte{malformedPayload})
	require.True(t, found)
	require.ErrorIs(t, err, ErrInvalidPayload)
}

func TestExtractPayloadPreservesConsensusPublicKeyBytes(t *testing.T) {
	expected := Payload{
		Height: 12,
		Votes: []*Votes{
			{
				ConsensusPublicKey: []byte{0x00, 0xff, 0x10, 0x80, 0x41},
				VoteExtension:      []byte("vote-extension"),
			},
		},
	}

	payloadBytes, err := expected.Marshal()
	require.NoError(t, err)

	markedPayload := append([]byte(VoteExtMarker), payloadBytes...)
	actual, found, err := extractPayload([][]byte{markedPayload})
	require.NoError(t, err)
	require.True(t, found)

	require.Equal(t, expected.Height, actual.Height)
	require.Len(t, actual.Votes, 1)
	require.Equal(t, expected.Votes[0].ConsensusPublicKey, actual.Votes[0].ConsensusPublicKey)
	require.Equal(t, expected.Votes[0].VoteExtension, actual.Votes[0].VoteExtension)
}
