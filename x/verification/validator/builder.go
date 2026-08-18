package validator

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"github.com/node101-io/pulsar-chain/x/verification/sidecar"
	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

var (
	ErrInvalidBuildRequest = errors.New("invalid verification build request")
	ErrProviderBusy        = errors.New("verification result provider is still busy")
	ErrProviderPanic       = errors.New("verification result provider panicked")
	ErrCommitmentDiverged  = errors.New("local commitment differs from consensus state")
)

type ConsensusReader interface {
	GetProofsAtHeight(context.Context, uint64) ([]verificationtypes.ProofEntry, error)
	GetCommitment(context.Context, []byte, uint64) ([]byte, bool, error)
}

type Identity struct {
	ChainID            string
	OperatorAddress    []byte
	ConsensusPublicKey []byte
}

type BuildOutcome struct {
	Payload *verificationtypes.VerificationVoteExtensionPayload
	Warning error
}

type Builder struct {
	reader   ConsensusReader
	provider sidecar.Provider
	store    StateStore
	random   io.Reader
	timeout  time.Duration

	mu       sync.Mutex
	state    State
	callGate chan struct{}
}

func NewBuilder(
	reader ConsensusReader,
	provider sidecar.Provider,
	store StateStore,
	random io.Reader,
	timeout time.Duration,
) (*Builder, error) {
	if reader == nil {
		return nil, fmt.Errorf("%w: missing consensus reader", ErrInvalidBuildRequest)
	}
	if provider == nil {
		provider = sidecar.DisabledProvider{}
	}
	if store == nil {
		store = NewMemoryStore()
	}
	if random == nil {
		random = cryptorand.Reader
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("%w: non-positive provider timeout", ErrInvalidBuildRequest)
	}
	state, err := store.Load()
	if err != nil {
		return nil, err
	}
	if err := ValidateState(state, true); err != nil {
		return nil, err
	}
	return &Builder{
		reader: reader, provider: provider, store: store, random: random, timeout: timeout,
		state: state.Clone(), callGate: make(chan struct{}, 1),
	}, nil
}

func (b *Builder) Build(ctx context.Context, identity Identity, targetHeight uint64) BuildOutcome {
	if b == nil {
		return BuildOutcome{Warning: ErrInvalidBuildRequest}
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if targetHeight < verificationtypes.CommitmentDeadline || identity.ChainID == "" ||
		len(identity.OperatorAddress) == 0 || len(identity.ConsensusPublicKey) == 0 {
		return BuildOutcome{Warning: ErrInvalidBuildRequest}
	}

	working, err := b.boundState(identity)
	if err != nil {
		return BuildOutcome{Warning: err}
	}
	stateChanged := working.Prune(targetHeight)
	revelations, warning := b.buildRevelations(ctx, working, targetHeight)

	var commitment []byte
	if existing, found := working.Commitment(targetHeight); found {
		commitment = bytes.Clone(existing.CommitmentRoot)
	} else {
		secret, built, buildWarning := b.buildCommitment(ctx, working, identity.OperatorAddress, targetHeight)
		warning = errors.Join(warning, buildWarning)
		if built {
			if err := working.PutCommitment(secret); err != nil {
				warning = errors.Join(warning, err)
			} else {
				stateChanged = true
				commitment = bytes.Clone(secret.CommitmentRoot)
			}
		}
	}

	if stateChanged {
		if err := ValidateState(working, false); err != nil {
			return BuildOutcome{Payload: payloadFor(targetHeight, nil, revelations), Warning: errors.Join(warning, err)}
		}
		if err := b.store.Save(working); err != nil {
			// A new root is never returned unless its exact preimage reached disk.
			return BuildOutcome{Payload: payloadFor(targetHeight, nil, revelations), Warning: errors.Join(warning, err)}
		}
		b.state = working.Clone()
	}

	return BuildOutcome{Payload: payloadFor(targetHeight, commitment, revelations), Warning: warning}
}

func (b *Builder) boundState(identity Identity) (State, error) {
	state := b.state.Clone()
	digest := sha256.Sum256(identity.ConsensusPublicKey)
	unbound := state.ChainID == "" && len(state.ValidatorOperatorAddress) == 0 && len(state.ValidatorConsensusKeyHash) == 0
	if unbound {
		state.ChainID = identity.ChainID
		state.ValidatorOperatorAddress = bytes.Clone(identity.OperatorAddress)
		state.ValidatorConsensusKeyHash = bytes.Clone(digest[:])
		return state, nil
	}
	if state.ChainID != identity.ChainID || !bytes.Equal(state.ValidatorOperatorAddress, identity.OperatorAddress) ||
		!bytes.Equal(state.ValidatorConsensusKeyHash, digest[:]) {
		return State{}, ErrStateBinding
	}
	return state, nil
}

func (b *Builder) buildRevelations(
	ctx context.Context,
	state State,
	targetHeight uint64,
) ([]verificationtypes.CommitmentRevelation, error) {
	revelations := make([]verificationtypes.CommitmentRevelation, 0, verificationtypes.MaxRevelationsPerBatch)
	var warning error
	for delta := verificationtypes.CommitmentDeadline; delta >= 1; delta-- {
		if targetHeight < delta {
			continue
		}
		commitmentHeight := targetHeight - delta
		secret, found := state.Commitment(commitmentHeight)
		if !found {
			continue
		}
		stored, exists, err := b.reader.GetCommitment(ctx, state.ValidatorOperatorAddress, commitmentHeight)
		if err != nil {
			warning = errors.Join(warning, err)
			continue
		}
		if !exists {
			continue
		}
		if len(stored) != len(secret.CommitmentRoot) || subtle.ConstantTimeCompare(stored, secret.CommitmentRoot) != 1 {
			warning = errors.Join(warning, fmt.Errorf("%w at height %d", ErrCommitmentDiverged, commitmentHeight))
			continue
		}
		revelation, ok := buildRevelation(targetHeight, secret)
		if ok {
			revelations = append(revelations, revelation)
		}
	}
	return revelations, warning
}

func buildRevelation(currentHeight uint64, secret CommitmentSecret) (verificationtypes.CommitmentRevelation, bool) {
	left, leftValue := revealLeaf(currentHeight, secret.Left)
	right, rightValue := revealLeaf(currentHeight, secret.Right)
	if !leftValue && !rightValue {
		return verificationtypes.CommitmentRevelation{}, false
	}
	return verificationtypes.CommitmentRevelation{
		CommitmentHeight: secret.CommitmentHeight,
		Left:             left,
		Right:            right,
	}, true
}

func revealLeaf(currentHeight uint64, secret LeafSecret) (verificationtypes.LeafRevelation, bool) {
	if verificationtypes.GetLeafTiming(currentHeight, secret.ProofHeight) != verificationtypes.LeafActive {
		return verificationtypes.LeafRevelation{
			Mode: verificationtypes.LeafRevealMode_LEAF_REVEAL_MODE_HASH_ONLY,
			Payload: &verificationtypes.LeafRevelation_LeafHash{
				LeafHash: bytes.Clone(secret.LeafHash),
			},
		}, false
	}
	return verificationtypes.LeafRevelation{
		Mode: verificationtypes.LeafRevealMode_LEAF_REVEAL_MODE_VALUE,
		Payload: &verificationtypes.LeafRevelation_Value{Value: &verificationtypes.RevealedLeaf{
			Salt: bytes.Clone(secret.Salt), Votes: append([]verificationtypes.ProofVote(nil), secret.Votes...),
		}},
	}, true
}

func (b *Builder) buildCommitment(
	ctx context.Context,
	state State,
	operator []byte,
	commitmentHeight uint64,
) (CommitmentSecret, bool, error) {
	leftHeight, rightHeight, err := verificationtypes.CommitmentProofHeights(commitmentHeight)
	if err != nil {
		return CommitmentSecret{}, false, err
	}
	leftProofs, err := b.reader.GetProofsAtHeight(ctx, leftHeight)
	if err != nil {
		return CommitmentSecret{}, false, err
	}
	rightProofs, err := b.reader.GetProofsAtHeight(ctx, rightHeight)
	if err != nil {
		return CommitmentSecret{}, false, err
	}

	excluded := make(map[uint32]struct{})
	if commitmentHeight > 0 {
		previous, found := state.Commitment(commitmentHeight - 1)
		if found {
			stored, exists, err := b.reader.GetCommitment(ctx, operator, previous.CommitmentHeight)
			if err != nil {
				return CommitmentSecret{}, false, err
			}
			if exists {
				if len(stored) != len(previous.CommitmentRoot) || subtle.ConstantTimeCompare(stored, previous.CommitmentRoot) != 1 {
					return CommitmentSecret{}, false, fmt.Errorf("%w at height %d", ErrCommitmentDiverged, previous.CommitmentHeight)
				}
				if previous.Right.ProofHeight == leftHeight {
					for _, vote := range previous.Right.Votes {
						excluded[vote.IndexInBlock] = struct{}{}
					}
				}
			}
		}
	}

	request, index, err := buildResultRequest(leftHeight, rightHeight, leftProofs, rightProofs, excluded)
	if err != nil || len(request) == 0 {
		return CommitmentSecret{}, false, err
	}
	results, err := b.callProvider(ctx, request)
	if err != nil {
		return CommitmentSecret{}, false, err
	}
	leftVotes, rightVotes, err := validateResults(index, results, leftHeight)
	if err != nil {
		return CommitmentSecret{}, false, err
	}
	if len(leftVotes) == 0 && len(rightVotes) == 0 {
		return CommitmentSecret{}, false, nil
	}
	return b.newCommitmentSecret(commitmentHeight, leftHeight, rightHeight, leftVotes, rightVotes)
}

func (b *Builder) newCommitmentSecret(
	commitmentHeight, leftHeight, rightHeight uint64,
	leftVotes, rightVotes []verificationtypes.ProofVote,
) (CommitmentSecret, bool, error) {
	leftSalt := make([]byte, verificationtypes.SaltSize)
	rightSalt := make([]byte, verificationtypes.SaltSize)
	if _, err := io.ReadFull(b.random, leftSalt); err != nil {
		return CommitmentSecret{}, false, err
	}
	if _, err := io.ReadFull(b.random, rightSalt); err != nil {
		return CommitmentSecret{}, false, err
	}
	leftHash, err := verificationtypes.ComputeLeafHash(leftSalt, leftVotes)
	if err != nil {
		return CommitmentSecret{}, false, err
	}
	rightHash, err := verificationtypes.ComputeLeafHash(rightSalt, rightVotes)
	if err != nil {
		return CommitmentSecret{}, false, err
	}
	root := verificationtypes.ComputeCommitmentRoot(leftHash, rightHash)
	secret := CommitmentSecret{
		CommitmentHeight: commitmentHeight,
		CommitmentRoot:   bytes.Clone(root[:]),
		Left: LeafSecret{
			ProofHeight: leftHeight, Salt: leftSalt, LeafHash: bytes.Clone(leftHash[:]),
			Votes: append([]verificationtypes.ProofVote(nil), leftVotes...),
		},
		Right: LeafSecret{
			ProofHeight: rightHeight, Salt: rightSalt, LeafHash: bytes.Clone(rightHash[:]),
			Votes: append([]verificationtypes.ProofVote(nil), rightVotes...),
		},
	}
	return secret, true, validateCommitmentSecret(secret)
}

func (b *Builder) callProvider(ctx context.Context, hashes [][]byte) ([]sidecar.VerificationResult, error) {
	select {
	case b.callGate <- struct{}{}:
	default:
		return nil, ErrProviderBusy
	}

	callCtx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()
	type response struct {
		results []sidecar.VerificationResult
		err     error
	}
	resultCh := make(chan response, 1)
	request := cloneHashes(hashes)
	go func() {
		defer func() { <-b.callGate }()
		out := response{}
		defer func() {
			if recovered := recover(); recovered != nil {
				out.err = fmt.Errorf("%w: %v", ErrProviderPanic, recovered)
			}
			resultCh <- out
		}()
		out.results, out.err = b.provider.GetVerificationResults(callCtx, request)
	}()

	select {
	case <-callCtx.Done():
		return nil, callCtx.Err()
	case out := <-resultCh:
		return out.results, out.err
	}
}

func payloadFor(
	targetHeight uint64,
	commitment []byte,
	revelations []verificationtypes.CommitmentRevelation,
) *verificationtypes.VerificationVoteExtensionPayload {
	if len(commitment) == 0 && len(revelations) == 0 {
		return nil
	}
	return &verificationtypes.VerificationVoteExtensionPayload{
		TargetHeight: targetHeight,
		Commitment:   bytes.Clone(commitment),
		Revelations:  append([]verificationtypes.CommitmentRevelation(nil), revelations...),
	}
}

func cloneHashes(hashes [][]byte) [][]byte {
	out := make([][]byte, len(hashes))
	for i := range hashes {
		out[i] = bytes.Clone(hashes[i])
	}
	return out
}

func TargetHeight(sourceHeight int64) (uint64, error) {
	if sourceHeight < 0 || sourceHeight == math.MaxInt64 {
		return 0, ErrInvalidBuildRequest
	}
	return uint64(sourceHeight + 1), nil
}
