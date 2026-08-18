package abci

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

func TestCompositeVoteExtensionCodec(t *testing.T) {
	extension := &CompositeVoteExtension{
		ProtocolVersion:     CompositeVoteExtensionVersion,
		TransitionSignature: []byte("transition"),
		VerificationPayload: &verificationtypes.VerificationVoteExtensionPayload{
			TargetHeight: 10,
			Commitment:   bytes.Repeat([]byte{1}, verificationtypes.CommitmentHashSize),
		},
	}

	encoded, err := encodeCompositeVoteExtension(extension)
	require.NoError(t, err)
	decoded, err := decodeCompositeVoteExtension(encoded)
	require.NoError(t, err)
	require.Equal(t, extension, decoded)

	extension.ProtocolVersion++
	encoded, err = extension.Marshal()
	require.NoError(t, err)
	_, err = decodeCompositeVoteExtension(encoded)
	require.ErrorIs(t, err, ErrInvalidCompositeVoteExtension)
}

func TestWorstCaseVerificationPayloadFitsHardCap(t *testing.T) {
	votes := make([]verificationtypes.ProofVote, verificationtypes.MaxVoteIndexExclusive)
	for i := range votes {
		votes[i] = verificationtypes.ProofVote{IndexInBlock: uint32(i), Result: i%2 == 0}
	}
	valueLeaf := func(salt byte) verificationtypes.LeafRevelation {
		return verificationtypes.LeafRevelation{
			Mode: verificationtypes.LeafRevealMode_LEAF_REVEAL_MODE_VALUE,
			Payload: &verificationtypes.LeafRevelation_Value{Value: &verificationtypes.RevealedLeaf{
				Salt: bytes.Repeat([]byte{salt}, verificationtypes.SaltSize), Votes: votes,
			}},
		}
	}
	hashLeaf := verificationtypes.LeafRevelation{
		Mode: verificationtypes.LeafRevealMode_LEAF_REVEAL_MODE_HASH_ONLY,
		Payload: &verificationtypes.LeafRevelation_LeafHash{
			LeafHash: bytes.Repeat([]byte{9}, verificationtypes.LeafHashSize),
		},
	}
	extension := &CompositeVoteExtension{
		ProtocolVersion:     CompositeVoteExtensionVersion,
		TransitionSignature: bytes.Repeat([]byte{1}, 64),
		VerificationPayload: &verificationtypes.VerificationVoteExtensionPayload{
			TargetHeight: 10,
			Commitment:   bytes.Repeat([]byte{2}, verificationtypes.CommitmentHashSize),
			Revelations: []verificationtypes.CommitmentRevelation{
				{CommitmentHeight: 7, Left: valueLeaf(1), Right: valueLeaf(2)},
				{CommitmentHeight: 8, Left: valueLeaf(3), Right: valueLeaf(4)},
				{CommitmentHeight: 9, Left: valueLeaf(5), Right: hashLeaf},
			},
		},
	}

	encoded, err := encodeCompositeVoteExtension(extension)
	require.NoError(t, err)
	require.NoError(t, validateVerificationPayloadStructure(extension.VerificationPayload, 10))
	require.Less(t, len(encoded), MaxCompositeVoteExtensionBytes)
}

func FuzzCompositeVoteExtensionDecoder(f *testing.F) {
	valid, err := encodeCompositeVoteExtension(&CompositeVoteExtension{
		ProtocolVersion:     CompositeVoteExtensionVersion,
		TransitionSignature: []byte("transition"),
	})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(valid)
	f.Add([]byte{})
	f.Add([]byte{0xff, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, encoded []byte) {
		decoded, err := decodeCompositeVoteExtension(encoded)
		if err != nil {
			return
		}
		reencoded, err := encodeCompositeVoteExtension(decoded)
		require.NoError(t, err)
		require.Equal(t, encoded, reencoded)
	})
}

func BenchmarkCompositeVoteExtensionCodec(b *testing.B) {
	for _, voteCount := range []int{0, 64, 256} {
		votes := make([]verificationtypes.ProofVote, voteCount)
		for i := range votes {
			votes[i] = verificationtypes.ProofVote{IndexInBlock: uint32(i), Result: i%2 == 0}
		}
		extension := &CompositeVoteExtension{
			ProtocolVersion:     CompositeVoteExtensionVersion,
			TransitionSignature: bytes.Repeat([]byte{1}, 64),
			VerificationPayload: &verificationtypes.VerificationVoteExtensionPayload{
				TargetHeight: 10,
				Commitment:   bytes.Repeat([]byte{2}, verificationtypes.CommitmentHashSize),
				Revelations: []verificationtypes.CommitmentRevelation{{
					CommitmentHeight: 9,
					Left: verificationtypes.LeafRevelation{
						Mode: verificationtypes.LeafRevealMode_LEAF_REVEAL_MODE_VALUE,
						Payload: &verificationtypes.LeafRevelation_Value{Value: &verificationtypes.RevealedLeaf{
							Salt: bytes.Repeat([]byte{3}, verificationtypes.SaltSize), Votes: votes,
						}},
					},
					Right: verificationtypes.LeafRevelation{
						Mode: verificationtypes.LeafRevealMode_LEAF_REVEAL_MODE_HASH_ONLY,
						Payload: &verificationtypes.LeafRevelation_LeafHash{
							LeafHash: bytes.Repeat([]byte{4}, verificationtypes.LeafHashSize),
						},
					},
				}},
			},
		}
		encoded, err := encodeCompositeVoteExtension(extension)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(fmt.Sprintf("%d_votes_%d_bytes", voteCount, len(encoded)), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(encoded)))
			for range b.N {
				if _, err := decodeCompositeVoteExtension(encoded); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSystemFramePacking(b *testing.B) {
	for _, validatorCount := range []int{10, 50, 100, 200} {
		payload := Payload{VoteExtensionHeight: 99}
		for i := 0; i < validatorCount; i++ {
			address := make([]byte, 20)
			binary.BigEndian.PutUint32(address[len(address)-4:], uint32(i+1))
			payload.VoteExtensions = append(payload.VoteExtensions, &PayloadVoteExtension{
				ConsensusPublicKey: bytes.Repeat([]byte{byte(i + 1)}, 32),
				VoteExtension:      bytes.Repeat([]byte{byte(i + 2)}, 64),
			})
			payload.VerificationEntries = append(payload.VerificationEntries, &PayloadVerificationEntry{
				ValidatorAddress:       address,
				SourceHeight:           99,
				CompositeVoteExtension: bytes.Repeat([]byte{byte(i + 3)}, 512),
				ExtensionSignature:     bytes.Repeat([]byte{byte(i + 4)}, 64),
			})
		}
		b.Run(fmt.Sprintf("%d_validators", validatorCount), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				fitted, err := fitVerificationEntries(payload, 100, 1<<20)
				if err != nil {
					b.Fatal(err)
				}
				if len(fitted.VerificationEntries) != validatorCount {
					b.Fatal("unexpected packed entry count")
				}
			}
		})
	}
}

func BenchmarkProcessProposalSignatureVerification(b *testing.B) {
	for _, validatorCount := range []int{10, 50, 100, 200} {
		keys := make([]interface {
			VerifySignature([]byte, []byte) bool
		}, validatorCount)
		extensions := make([][]byte, validatorCount)
		signatures := make([][]byte, validatorCount)
		for i := 0; i < validatorCount; i++ {
			key, _ := knownConsensusValidator(b, 10)
			extension := bytes.Repeat([]byte{byte(i + 1)}, 256)
			vote := signedExtendedVote(b, key, extension, 99, 0, "benchmark-chain")
			keys[i] = key.PubKey()
			extensions[i] = extension
			signatures[i] = vote.ExtensionSignature
		}
		b.Run(fmt.Sprintf("%d_validators", validatorCount), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				for i := range keys {
					if err := verifyCometVoteExtensionSignature(
						"benchmark-chain", keys[i], 99, 0, extensions[i], signatures[i],
					); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
