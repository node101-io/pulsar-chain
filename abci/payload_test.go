package abci

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractPayloadReportsMissingPayload(t *testing.T) {
	payload, found, err := extractPayload(nil)
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, payload.VoteExtensions)

	payload, found, err = extractPayload([][]byte{[]byte("user-tx")})
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, payload.VoteExtensions)
}

func TestExtractPayloadRejectsInvalidPayloadBody(t *testing.T) {
	_, found, err := extractPayload([][]byte{voteExtMarkerBytes})
	require.True(t, found)
	require.ErrorIs(t, err, ErrInvalidPayload)

	malformedPayload := append(voteExtMarkerBytes[:len(voteExtMarkerBytes):len(voteExtMarkerBytes)], []byte{0xff, 0xff, 0xff}...)
	_, found, err = extractPayload([][]byte{malformedPayload})
	require.True(t, found)
	require.ErrorIs(t, err, ErrInvalidPayload)
}

func TestExtractPayloadRejectsReservedMarkerOutsideFirstSlot(t *testing.T) {
	valid := markedPayloadTx(t, Payload{VoteExtensionHeight: 9})
	_, found, err := extractPayload([][]byte{valid, []byte("user-tx"), valid})
	require.True(t, found)
	require.ErrorIs(t, err, ErrInvalidPayload)

	_, found, err = extractPayload([][]byte{[]byte("user-tx"), valid})
	require.True(t, found)
	require.ErrorIs(t, err, ErrInvalidPayload)
}

func TestExtractPayloadPreservesConsensusPublicKeyBytes(t *testing.T) {
	expected := Payload{
		VoteExtensionHeight: 12,
		VoteExtensions: []*PayloadVoteExtension{
			{
				ConsensusPublicKey: []byte{0x00, 0xff, 0x10, 0x80, 0x41},
				VoteExtension:      []byte("vote-extension"),
			},
		},
	}

	payloadBytes, err := expected.Marshal()
	require.NoError(t, err)

	markedPayload := append(voteExtMarkerBytes[:len(voteExtMarkerBytes):len(voteExtMarkerBytes)], payloadBytes...)
	actual, found, err := extractPayload([][]byte{markedPayload})
	require.NoError(t, err)
	require.True(t, found)

	require.Equal(t, expected.VoteExtensionHeight, actual.VoteExtensionHeight)
	require.Len(t, actual.VoteExtensions, 1)
	require.Equal(t, expected.VoteExtensions[0].ConsensusPublicKey, actual.VoteExtensions[0].ConsensusPublicKey)
	require.Equal(t, expected.VoteExtensions[0].VoteExtension, actual.VoteExtensions[0].VoteExtension)
}

func TestValidatePayloadHeight(t *testing.T) {
	require.NoError(t, validatePayloadHeight(Payload{VoteExtensionHeight: 12}, 12))
	require.ErrorIs(t, validatePayloadHeight(Payload{VoteExtensionHeight: 11}, 12), ErrInvalidPayloadHeight)
}
