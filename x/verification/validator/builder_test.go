package validator

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/verification/sidecar"
	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

type readerStub struct {
	mu          sync.Mutex
	proofs      map[uint64][]verificationtypes.ProofEntry
	commitments map[string][]byte
}

func newReaderStub() *readerStub {
	return &readerStub{
		proofs: make(map[uint64][]verificationtypes.ProofEntry), commitments: make(map[string][]byte),
	}
}

func (r *readerStub) GetProofsAtHeight(_ context.Context, height uint64) ([]verificationtypes.ProofEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	proofs := r.proofs[height]
	out := make([]verificationtypes.ProofEntry, len(proofs))
	for i, proof := range proofs {
		out[i] = verificationtypes.ProofEntry{
			Key: proof.Key,
			Record: verificationtypes.ProofRecord{
				ProofHash: bytes.Clone(proof.Record.ProofHash), ProofType: proof.Record.ProofType,
			},
		}
	}
	return out, nil
}

func (r *readerStub) GetCommitment(_ context.Context, validator []byte, height uint64) ([]byte, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	root, found := r.commitments[commitmentKey(validator, height)]
	return bytes.Clone(root), found, nil
}

func (r *readerStub) setProofs(height uint64, count int, seed byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	proofs := make([]verificationtypes.ProofEntry, count)
	for i := range proofs {
		hash := bytes.Repeat([]byte{seed + byte(i)}, verificationtypes.ProofHashSize)
		proofs[i] = verificationtypes.ProofEntry{
			Key:    verificationtypes.ProofKey{SubmissionHeight: height, IndexInBlock: uint32(i)},
			Record: verificationtypes.ProofRecord{ProofHash: hash, ProofType: 1},
		}
	}
	r.proofs[height] = proofs
}

func (r *readerStub) setCommitment(operator []byte, height uint64, root []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commitments[commitmentKey(operator, height)] = bytes.Clone(root)
}

func commitmentKey(operator []byte, height uint64) string {
	var encodedHeight [8]byte
	binary.BigEndian.PutUint64(encodedHeight[:], height)
	return string(operator) + string(encodedHeight[:])
}

func testIdentity() Identity {
	return Identity{
		ChainID: "verification-test", OperatorAddress: bytes.Repeat([]byte{0x11}, 20),
		ConsensusPublicKey: bytes.Repeat([]byte{0x22}, 32),
	}
}

func newTestBuilder(
	t *testing.T,
	reader *readerStub,
	provider sidecar.Provider,
	store StateStore,
) *Builder {
	t.Helper()
	random := bytes.NewReader(bytes.Repeat([]byte{0xa5}, 4096))
	builder, err := NewBuilder(reader, provider, store, random, 25*time.Millisecond)
	require.NoError(t, err)
	return builder
}

func TestBuilderPersistsBeforeReturningAndReusesSameHeight(t *testing.T) {
	reader := newReaderStub()
	reader.setProofs(7, 1, 0x30)
	reader.setProofs(8, 2, 0x40)
	providerCalls := 0
	provider := sidecar.ProviderFunc(func(_ context.Context, hashes [][]byte) ([]sidecar.VerificationResult, error) {
		providerCalls++
		require.Len(t, hashes, 3)
		return []sidecar.VerificationResult{
			{ProofHash: bytes.Clone(hashes[2]), Result: sidecar.ResultInvalid},
			{ProofHash: bytes.Clone(hashes[0]), Result: sidecar.ResultValid},
		}, nil
	})
	store := NewMemoryStore()
	builder := newTestBuilder(t, reader, provider, store)

	first := builder.Build(context.Background(), testIdentity(), 10)
	require.NoError(t, first.Warning)
	require.NotNil(t, first.Payload)
	require.Len(t, first.Payload.Commitment, verificationtypes.CommitmentHashSize)
	require.Equal(t, 1, providerCalls)

	persisted, err := store.Load()
	require.NoError(t, err)
	secret, found := persisted.Commitment(10)
	require.True(t, found)
	require.Equal(t, first.Payload.Commitment, secret.CommitmentRoot)
	require.Equal(t, []verificationtypes.ProofVote{{IndexInBlock: 0, Result: true}}, secret.Left.Votes)
	require.Equal(t, []verificationtypes.ProofVote{{IndexInBlock: 1, Result: false}}, secret.Right.Votes)

	second := builder.Build(context.Background(), testIdentity(), 10)
	require.NoError(t, second.Warning)
	require.Equal(t, first.Payload.Commitment, second.Payload.Commitment)
	require.Equal(t, 1, providerCalls)
}

func TestBuilderNeverReturnsUnsavedCommitment(t *testing.T) {
	reader := newReaderStub()
	reader.setProofs(8, 1, 0x40)
	provider := sidecar.ProviderFunc(func(_ context.Context, hashes [][]byte) ([]sidecar.VerificationResult, error) {
		return []sidecar.VerificationResult{{ProofHash: bytes.Clone(hashes[0]), Result: sidecar.ResultValid}}, nil
	})
	store := NewMemoryStore()
	store.SetSaveError(errors.New("disk unavailable"))
	builder := newTestBuilder(t, reader, provider, store)

	outcome := builder.Build(context.Background(), testIdentity(), 10)
	require.ErrorContains(t, outcome.Warning, "disk unavailable")
	require.Nil(t, outcome.Payload)
	state, err := store.Load()
	require.NoError(t, err)
	require.Empty(t, state.Commitments)
}

func TestBuilderRestartRevealsWithoutSidecar(t *testing.T) {
	reader := newReaderStub()
	reader.setProofs(7, 1, 0x30)
	provider := sidecar.ProviderFunc(func(_ context.Context, hashes [][]byte) ([]sidecar.VerificationResult, error) {
		return []sidecar.VerificationResult{{ProofHash: bytes.Clone(hashes[0]), Result: sidecar.ResultValid}}, nil
	})
	statePath := filepath.Join(t.TempDir(), "private", "verification_commitment_state.json")
	store, err := NewFileStore(statePath)
	require.NoError(t, err)
	firstBuilder := newTestBuilder(t, reader, provider, store)
	committed := firstBuilder.Build(context.Background(), testIdentity(), 10)
	require.NoError(t, committed.Warning)
	require.NotNil(t, committed.Payload)
	reader.setCommitment(testIdentity().OperatorAddress, 10, committed.Payload.Commitment)

	restarted, err := NewBuilder(reader, sidecar.DisabledProvider{}, store, bytes.NewReader(nil), 25*time.Millisecond)
	require.NoError(t, err)
	revealed := restarted.Build(context.Background(), testIdentity(), 11)
	require.NoError(t, revealed.Warning)
	require.NotNil(t, revealed.Payload)
	require.Empty(t, revealed.Payload.Commitment)
	require.Len(t, revealed.Payload.Revelations, 1)
	require.Equal(t, uint64(10), revealed.Payload.Revelations[0].CommitmentHeight)
	require.Equal(t, verificationtypes.LeafRevealMode_LEAF_REVEAL_MODE_VALUE, revealed.Payload.Revelations[0].Left.Mode)
	require.Equal(t, verificationtypes.LeafRevealMode_LEAF_REVEAL_MODE_HASH_ONLY, revealed.Payload.Revelations[0].Right.Mode)
}

func TestBuilderHPlusTwoAndHPlusThreeUseDifferentSubsets(t *testing.T) {
	reader := newReaderStub()
	reader.setProofs(10, 3, 0x50)
	reader.setProofs(11, 1, 0x60)
	call := 0
	provider := sidecar.ProviderFunc(func(_ context.Context, hashes [][]byte) ([]sidecar.VerificationResult, error) {
		call++
		if call == 1 {
			return []sidecar.VerificationResult{
				{ProofHash: bytes.Clone(hashes[0]), Result: sidecar.ResultValid},
				{ProofHash: bytes.Clone(hashes[2]), Result: sidecar.ResultInvalid},
			}, nil
		}
		var results []sidecar.VerificationResult
		for _, hash := range hashes {
			if bytes.Equal(hash, reader.proofs[10][1].Record.ProofHash) {
				results = append(results, sidecar.VerificationResult{ProofHash: bytes.Clone(hash), Result: sidecar.ResultValid})
			}
		}
		return results, nil
	})
	store := NewMemoryStore()
	builder := newTestBuilder(t, reader, provider, store)

	first := builder.Build(context.Background(), testIdentity(), 12)
	require.NoError(t, first.Warning)
	require.NotNil(t, first.Payload)
	reader.setCommitment(testIdentity().OperatorAddress, 12, first.Payload.Commitment)

	second := builder.Build(context.Background(), testIdentity(), 13)
	require.NoError(t, second.Warning)
	require.NotNil(t, second.Payload)
	state, err := store.Load()
	require.NoError(t, err)
	secret, found := state.Commitment(13)
	require.True(t, found)
	require.Equal(t, []verificationtypes.ProofVote{{IndexInBlock: 1, Result: true}}, secret.Left.Votes)
}

func TestBuilderProviderFailureKeepsPersistedRevelation(t *testing.T) {
	reader := newReaderStub()
	reader.setProofs(7, 1, 0x30)
	provider := sidecar.ProviderFunc(func(_ context.Context, hashes [][]byte) ([]sidecar.VerificationResult, error) {
		return []sidecar.VerificationResult{{ProofHash: bytes.Clone(hashes[0]), Result: sidecar.ResultValid}}, nil
	})
	store := NewMemoryStore()
	builder := newTestBuilder(t, reader, provider, store)
	first := builder.Build(context.Background(), testIdentity(), 10)
	require.NoError(t, first.Warning)
	reader.setCommitment(testIdentity().OperatorAddress, 10, first.Payload.Commitment)
	reader.setProofs(8, 1, 0x40)

	failing, err := NewBuilder(reader, sidecar.ProviderFunc(func(context.Context, [][]byte) ([]sidecar.VerificationResult, error) {
		return nil, errors.New("sidecar offline")
	}), store, bytes.NewReader(bytes.Repeat([]byte{1}, 64)), 25*time.Millisecond)
	require.NoError(t, err)
	outcome := failing.Build(context.Background(), testIdentity(), 11)
	require.ErrorContains(t, outcome.Warning, "sidecar offline")
	require.NotNil(t, outcome.Payload)
	require.Empty(t, outcome.Payload.Commitment)
	require.Len(t, outcome.Payload.Revelations, 1)
}

func TestBuilderRejectsMalformedSidecarResponse(t *testing.T) {
	reader := newReaderStub()
	reader.setProofs(8, 1, 0x40)
	provider := sidecar.ProviderFunc(func(context.Context, [][]byte) ([]sidecar.VerificationResult, error) {
		return []sidecar.VerificationResult{{ProofHash: bytes.Repeat([]byte{9}, verificationtypes.ProofHashSize), Result: sidecar.ResultValid}}, nil
	})
	builder := newTestBuilder(t, reader, provider, NewMemoryStore())
	outcome := builder.Build(context.Background(), testIdentity(), 10)
	require.ErrorContains(t, outcome.Warning, "unrequested")
	require.Nil(t, outcome.Payload)
}

func TestBuilderPrunesExpiredLocalRecords(t *testing.T) {
	reader := newReaderStub()
	reader.setProofs(7, 1, 0x30)
	provider := sidecar.ProviderFunc(func(_ context.Context, hashes [][]byte) ([]sidecar.VerificationResult, error) {
		return []sidecar.VerificationResult{{ProofHash: bytes.Clone(hashes[0]), Result: sidecar.ResultValid}}, nil
	})
	store := NewMemoryStore()
	builder := newTestBuilder(t, reader, provider, store)
	first := builder.Build(context.Background(), testIdentity(), 10)
	require.NoError(t, first.Warning)
	require.NotNil(t, first.Payload)

	pruned := builder.Build(context.Background(), testIdentity(), 14)
	require.Nil(t, pruned.Payload)
	state, err := store.Load()
	require.NoError(t, err)
	require.Empty(t, state.Commitments)
}

func TestBuilderPreviousUncommittedRecordDoesNotExcludeVotes(t *testing.T) {
	reader := newReaderStub()
	reader.setProofs(10, 1, 0x50)
	provider := sidecar.ProviderFunc(func(_ context.Context, hashes [][]byte) ([]sidecar.VerificationResult, error) {
		return []sidecar.VerificationResult{{ProofHash: bytes.Clone(hashes[0]), Result: sidecar.ResultValid}}, nil
	})
	store := NewMemoryStore()
	builder := newTestBuilder(t, reader, provider, store)

	first := builder.Build(context.Background(), testIdentity(), 12)
	require.NoError(t, first.Warning)
	require.NotNil(t, first.Payload)

	second := builder.Build(context.Background(), testIdentity(), 13)
	require.NoError(t, second.Warning)
	require.NotNil(t, second.Payload)
	state, err := store.Load()
	require.NoError(t, err)
	secret, found := state.Commitment(13)
	require.True(t, found)
	require.Equal(t, []verificationtypes.ProofVote{{IndexInBlock: 0, Result: true}}, secret.Left.Votes)
}

func TestBuilderDetectsStateBindingAndCommitmentDivergence(t *testing.T) {
	reader := newReaderStub()
	reader.setProofs(7, 1, 0x30)
	provider := sidecar.ProviderFunc(func(_ context.Context, hashes [][]byte) ([]sidecar.VerificationResult, error) {
		return []sidecar.VerificationResult{{ProofHash: bytes.Clone(hashes[0]), Result: sidecar.ResultValid}}, nil
	})
	store := NewMemoryStore()
	builder := newTestBuilder(t, reader, provider, store)
	first := builder.Build(context.Background(), testIdentity(), 10)
	require.NoError(t, first.Warning)

	otherIdentity := testIdentity()
	otherIdentity.ChainID = "other-chain"
	mismatch := builder.Build(context.Background(), otherIdentity, 11)
	require.ErrorIs(t, mismatch.Warning, ErrStateBinding)
	require.Nil(t, mismatch.Payload)

	reader.setCommitment(testIdentity().OperatorAddress, 10, bytes.Repeat([]byte{0xff}, verificationtypes.CommitmentHashSize))
	diverged := builder.Build(context.Background(), testIdentity(), 11)
	require.ErrorIs(t, diverged.Warning, ErrCommitmentDiverged)
	require.Nil(t, diverged.Payload)
}

func TestBuilderProviderTimeoutPanicAndBusyAreBounded(t *testing.T) {
	reader := newReaderStub()
	reader.setProofs(8, 1, 0x40)

	t.Run("panic", func(t *testing.T) {
		builder := newTestBuilder(t, reader, sidecar.ProviderFunc(func(context.Context, [][]byte) ([]sidecar.VerificationResult, error) {
			panic("boom")
		}), NewMemoryStore())
		outcome := builder.Build(context.Background(), testIdentity(), 10)
		require.ErrorIs(t, outcome.Warning, ErrProviderPanic)
		require.Nil(t, outcome.Payload)
	})

	t.Run("timeout and busy", func(t *testing.T) {
		release := make(chan struct{})
		started := make(chan struct{})
		provider := sidecar.ProviderFunc(func(context.Context, [][]byte) ([]sidecar.VerificationResult, error) {
			close(started)
			<-release
			return nil, nil
		})
		builder := newTestBuilder(t, reader, provider, NewMemoryStore())
		first := builder.Build(context.Background(), testIdentity(), 10)
		require.ErrorIs(t, first.Warning, context.DeadlineExceeded)
		<-started
		second := builder.Build(context.Background(), testIdentity(), 10)
		require.ErrorIs(t, second.Warning, ErrProviderBusy)
		close(release)
	})
}

func TestTargetHeight(t *testing.T) {
	target, err := TargetHeight(9)
	require.NoError(t, err)
	require.Equal(t, uint64(10), target)
	for _, source := range []int64{-1, int64(^uint64(0) >> 1)} {
		_, err := TargetHeight(source)
		require.ErrorIs(t, err, ErrInvalidBuildRequest)
	}
}
