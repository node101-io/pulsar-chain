package validator

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/node101-io/pulsar-chain/x/verification/sidecar"
	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

func BenchmarkValidateResults512(b *testing.B) {
	requested := make(map[string]requestedProof, verificationtypes.MaxVoteIndexExclusive*2)
	results := make([]sidecar.VerificationResult, 0, verificationtypes.MaxVoteIndexExclusive*2)
	for side := uint64(0); side < 2; side++ {
		for index := uint32(0); index < verificationtypes.MaxVoteIndexExclusive; index++ {
			hash := benchmarkHash(side, index)
			requested[string(hash)] = requestedProof{height: 7 + side, index: index}
			results = append(results, sidecar.VerificationResult{ProofHash: hash, Result: sidecar.ResultValid})
		}
	}
	b.ResetTimer()
	for range b.N {
		if _, _, err := validateResults(requested, results, 7); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildTwoFullLeaves(b *testing.B) {
	votes := make([]verificationtypes.ProofVote, verificationtypes.MaxVoteIndexExclusive)
	for index := range votes {
		votes[index] = verificationtypes.ProofVote{IndexInBlock: uint32(index), Result: index%2 == 0}
	}
	builder := &Builder{random: constantReader(0xa5)}
	b.ResetTimer()
	for range b.N {
		if _, _, err := builder.newCommitmentSecret(10, 7, 8, votes, votes); err != nil {
			b.Fatal(err)
		}
	}
}

type constantReader byte

func (r constantReader) Read(buffer []byte) (int, error) {
	for i := range buffer {
		buffer[i] = byte(r)
	}
	return len(buffer), nil
}

func BenchmarkFileStoreSaveAndLoadFourRecords(b *testing.B) {
	state := benchmarkState(b, 4)
	store, err := NewFileStore(filepath.Join(b.TempDir(), "private", "state.json"))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		if err := store.Save(state); err != nil {
			b.Fatal(err)
		}
		if _, err := store.Load(); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkState(tb testing.TB, count int) State {
	tb.Helper()
	state := State{
		Version: StateVersion, ChainID: "benchmark-chain",
		ValidatorOperatorAddress: bytes.Repeat([]byte{1}, 20), ValidatorConsensusKeyHash: bytes.Repeat([]byte{2}, 32),
	}
	for offset := 0; offset < count; offset++ {
		height := uint64(10 + offset)
		leftVotes := make([]verificationtypes.ProofVote, verificationtypes.MaxVoteIndexExclusive)
		rightVotes := make([]verificationtypes.ProofVote, verificationtypes.MaxVoteIndexExclusive)
		for index := range leftVotes {
			leftVotes[index] = verificationtypes.ProofVote{IndexInBlock: uint32(index), Result: true}
			rightVotes[index] = verificationtypes.ProofVote{IndexInBlock: uint32(index), Result: false}
		}
		leftSalt := bytes.Repeat([]byte{byte(offset + 1)}, verificationtypes.SaltSize)
		rightSalt := bytes.Repeat([]byte{byte(offset + 11)}, verificationtypes.SaltSize)
		leftHash, err := verificationtypes.ComputeLeafHash(leftSalt, leftVotes)
		if err != nil {
			tb.Fatal(err)
		}
		rightHash, err := verificationtypes.ComputeLeafHash(rightSalt, rightVotes)
		if err != nil {
			tb.Fatal(err)
		}
		root := verificationtypes.ComputeCommitmentRoot(leftHash, rightHash)
		state.Commitments = append(state.Commitments, CommitmentSecret{
			CommitmentHeight: height, CommitmentRoot: bytes.Clone(root[:]),
			Left:  LeafSecret{ProofHeight: height - 3, Salt: leftSalt, LeafHash: bytes.Clone(leftHash[:]), Votes: leftVotes},
			Right: LeafSecret{ProofHeight: height - 2, Salt: rightSalt, LeafHash: bytes.Clone(rightHash[:]), Votes: rightVotes},
		})
	}
	return state
}

func benchmarkHash(side uint64, index uint32) []byte {
	hash := make([]byte, verificationtypes.ProofHashSize)
	hash[0] = byte(side)
	hash[1] = byte(index >> 8)
	hash[2] = byte(index)
	return hash
}
