package types

import (
	"encoding/binary"

	"github.com/node101-io/mina-signer-go/keys"
)

type SecondaryKey struct {
	SecretKey *keys.PrivateKey
	PublicKey *keys.PublicKey
}

const DevnetNetworkID = "devnet"

var _ binary.ByteOrder

const (
	// VoteExtKeyPrefix is the prefix to retrieve all VoteExt
	VoteExtKeyPrefix = "VoteExt/value/"

	// VoteExtIndexKeyPrefix is the prefix for height-based index mapping
	VoteExtIndexKeyPrefix = "VoteExtIndex/value/"
)

var VoteExtMarker = []byte("VOTEEXT:")

// VoteExtKey returns the store key to retrieve a VoteExt from the index fields
func VoteExtKey(
	index string,
) []byte {
	var key []byte

	indexBytes := []byte(index)
	key = append(key, indexBytes...)
	key = append(key, []byte("/")...)

	return key
}

// VoteExtIndexKey returns the store key for height-based index mapping
func VoteExtIndexKey(height uint64) []byte {
	return binary.BigEndian.AppendUint64(nil, height)
}
