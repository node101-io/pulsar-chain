package validator

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

const (
	// StateVersion protects the on-disk journal format from silent reinterpretation.
	StateVersion            uint32 = 1
	maxChainIDBytes         int    = 128
	maxOperatorAddressBytes int    = 255
)

var (
	// ErrInvalidLocalState reports malformed or internally inconsistent journal data.
	ErrInvalidLocalState = errors.New("invalid verification local state")
	// ErrStateBinding reports reuse under a different chain or validator identity.
	ErrStateBinding = errors.New("verification local state binding mismatch")
)

// State is bounded validator-local state. It is not exported in genesis and
// is never replicated through consensus. Validators may therefore have
// different pending secrets without changing app hash; consensus sees only the
// roots and revelations they choose to submit.
type State struct {
	Version                   uint32
	ChainID                   string
	ValidatorOperatorAddress  []byte
	ValidatorConsensusKeyHash []byte
	Commitments               []CommitmentSecret
}

// CommitmentSecret stores the exact root preimage needed for later revelation.
// Losing it after the root is signed means the validator cannot contribute
// those votes, which is why the builder persists it before returning a payload.
type CommitmentSecret struct {
	CommitmentHeight uint64
	CommitmentRoot   []byte
	Left             LeafSecret
	Right            LeafSecret
}

// LeafSecret stores one proof-height vote set and its random salt. LeafHash is
// redundant by design and is recomputed during validation to detect corruption.
type LeafSecret struct {
	ProofHeight uint64
	Salt        []byte
	LeafHash    []byte
	Votes       []verificationtypes.ProofVote
}

type diskState struct {
	Version                   uint32                 `json:"version"`
	ChainID                   string                 `json:"chain_id"`
	ValidatorOperatorAddress  string                 `json:"validator_operator_address"`
	ValidatorConsensusKeyHash string                 `json:"validator_consensus_key_hash"`
	Commitments               []diskCommitmentSecret `json:"commitments"`
}

type diskCommitmentSecret struct {
	CommitmentHeight uint64         `json:"commitment_height"`
	CommitmentRoot   string         `json:"commitment_root"`
	Left             diskLeafSecret `json:"left"`
	Right            diskLeafSecret `json:"right"`
}

type diskLeafSecret struct {
	ProofHeight uint64                        `json:"proof_height"`
	Salt        string                        `json:"salt"`
	LeafHash    string                        `json:"leaf_hash"`
	Votes       []verificationtypes.ProofVote `json:"votes"`
}

// EmptyState returns a versioned journal that has not yet been bound to a
// validator identity.
func EmptyState() State {
	return State{Version: StateVersion}
}

// Clone returns a deep copy so callers cannot mutate persisted byte slices.
func (s State) Clone() State {
	out := State{
		Version:                   s.Version,
		ChainID:                   s.ChainID,
		ValidatorOperatorAddress:  bytes.Clone(s.ValidatorOperatorAddress),
		ValidatorConsensusKeyHash: bytes.Clone(s.ValidatorConsensusKeyHash),
		Commitments:               make([]CommitmentSecret, len(s.Commitments)),
	}
	for i := range s.Commitments {
		out.Commitments[i] = s.Commitments[i].clone()
	}
	return out
}

func (s CommitmentSecret) clone() CommitmentSecret {
	return CommitmentSecret{
		CommitmentHeight: s.CommitmentHeight,
		CommitmentRoot:   bytes.Clone(s.CommitmentRoot),
		Left:             s.Left.clone(),
		Right:            s.Right.clone(),
	}
}

func (s LeafSecret) clone() LeafSecret {
	return LeafSecret{
		ProofHeight: s.ProofHeight,
		Salt:        bytes.Clone(s.Salt),
		LeafHash:    bytes.Clone(s.LeafHash),
		Votes:       append([]verificationtypes.ProofVote(nil), s.Votes...),
	}
}

// Commitment performs a binary search over strictly increasing heights and
// returns a defensive copy.
func (s State) Commitment(height uint64) (CommitmentSecret, bool) {
	index := sort.Search(len(s.Commitments), func(i int) bool {
		return s.Commitments[i].CommitmentHeight >= height
	})
	if index == len(s.Commitments) || s.Commitments[index].CommitmentHeight != height {
		return CommitmentSecret{}, false
	}
	return s.Commitments[index].clone(), true
}

// PutCommitment inserts one secret while preserving strict height order.
func (s *State) PutCommitment(secret CommitmentSecret) error {
	if s == nil {
		return ErrInvalidLocalState
	}
	index := sort.Search(len(s.Commitments), func(i int) bool {
		return s.Commitments[i].CommitmentHeight >= secret.CommitmentHeight
	})
	if index < len(s.Commitments) && s.Commitments[index].CommitmentHeight == secret.CommitmentHeight {
		return fmt.Errorf("%w: commitment height %d already exists", ErrInvalidLocalState, secret.CommitmentHeight)
	}
	s.Commitments = append(s.Commitments, CommitmentSecret{})
	copy(s.Commitments[index+1:], s.Commitments[index:])
	s.Commitments[index] = secret.clone()
	return nil
}

// Prune removes secrets after their final revelation opportunity. The journal
// therefore remains bounded to the current commitment window. Retaining older
// salts adds secret-management risk without enabling any legal chain action.
func (s *State) Prune(targetHeight uint64) bool {
	if s == nil || len(s.Commitments) == 0 {
		return false
	}
	firstActive := 0
	for firstActive < len(s.Commitments) {
		height := s.Commitments[firstActive].CommitmentHeight
		if targetHeight <= height || targetHeight-height <= verificationtypes.CommitmentDeadline {
			break
		}
		firstActive++
	}
	if firstActive == 0 {
		return false
	}
	remaining := make([]CommitmentSecret, len(s.Commitments)-firstActive)
	for i := range remaining {
		remaining[i] = s.Commitments[firstActive+i].clone()
	}
	s.Commitments = remaining
	return true
}

// ValidateState verifies identity binding, boundedness, ordering, every leaf
// hash, and every commitment root before local secrets are trusted. The file is
// treated as untrusted restart input because manual edits, partial copies, or
// disk corruption must not lead to a different revealed preimage.
func ValidateState(s State, allowUnbound bool) error {
	if s.Version != StateVersion {
		return fmt.Errorf("%w: unsupported version %d", ErrInvalidLocalState, s.Version)
	}
	unbound := s.ChainID == "" && len(s.ValidatorOperatorAddress) == 0 && len(s.ValidatorConsensusKeyHash) == 0
	if unbound {
		if allowUnbound && len(s.Commitments) == 0 {
			return nil
		}
		return fmt.Errorf("%w: missing state binding", ErrInvalidLocalState)
	}
	if s.ChainID == "" || len(s.ChainID) > maxChainIDBytes || len(s.ValidatorOperatorAddress) == 0 ||
		len(s.ValidatorOperatorAddress) > maxOperatorAddressBytes || len(s.ValidatorConsensusKeyHash) != 32 {
		return fmt.Errorf("%w: incomplete state binding", ErrInvalidLocalState)
	}
	if len(s.Commitments) > verificationtypes.MaxRevelationsPerBatch+1 {
		return fmt.Errorf("%w: too many commitment records", ErrInvalidLocalState)
	}

	var previous uint64
	for i, secret := range s.Commitments {
		if i > 0 && secret.CommitmentHeight <= previous {
			return fmt.Errorf("%w: commitment heights are not strictly increasing", ErrInvalidLocalState)
		}
		previous = secret.CommitmentHeight
		if err := validateCommitmentSecret(secret); err != nil {
			return err
		}
	}
	return nil
}

func validateCommitmentSecret(secret CommitmentSecret) error {
	leftHeight, rightHeight, err := verificationtypes.CommitmentProofHeights(secret.CommitmentHeight)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidLocalState, err)
	}
	if secret.Left.ProofHeight != leftHeight || secret.Right.ProofHeight != rightHeight {
		return fmt.Errorf("%w: commitment proof heights", ErrInvalidLocalState)
	}
	// Recompute the complete commitment from stored preimages rather than
	// trusting redundant hashes in the file.
	leftHash, err := validateLeafSecret(secret.Left)
	if err != nil {
		return err
	}
	rightHash, err := validateLeafSecret(secret.Right)
	if err != nil {
		return err
	}
	if len(secret.CommitmentRoot) != verificationtypes.CommitmentHashSize {
		return fmt.Errorf("%w: commitment root length", ErrInvalidLocalState)
	}
	root := verificationtypes.ComputeCommitmentRoot(leftHash, rightHash)
	if !bytes.Equal(root[:], secret.CommitmentRoot) {
		return fmt.Errorf("%w: commitment root mismatch", ErrInvalidLocalState)
	}
	return nil
}

func validateLeafSecret(secret LeafSecret) ([verificationtypes.LeafHashSize]byte, error) {
	var empty [verificationtypes.LeafHashSize]byte
	if len(secret.Salt) != verificationtypes.SaltSize || len(secret.LeafHash) != verificationtypes.LeafHashSize {
		return empty, fmt.Errorf("%w: leaf hash or salt length", ErrInvalidLocalState)
	}
	if err := verificationtypes.ValidateCanonicalVotes(secret.Votes); err != nil {
		return empty, fmt.Errorf("%w: %v", ErrInvalidLocalState, err)
	}
	hash, err := verificationtypes.ComputeLeafHash(secret.Salt, secret.Votes)
	if err != nil {
		return empty, fmt.Errorf("%w: %v", ErrInvalidLocalState, err)
	}
	if !bytes.Equal(hash[:], secret.LeafHash) {
		return empty, fmt.Errorf("%w: leaf hash mismatch", ErrInvalidLocalState)
	}
	return hash, nil
}

func encodeState(s State) ([]byte, error) {
	// Never serialize a state that the builder would refuse to load.
	if err := ValidateState(s, false); err != nil {
		return nil, err
	}
	disk := diskState{
		Version:                   s.Version,
		ChainID:                   s.ChainID,
		ValidatorOperatorAddress:  hex.EncodeToString(s.ValidatorOperatorAddress),
		ValidatorConsensusKeyHash: hex.EncodeToString(s.ValidatorConsensusKeyHash),
		Commitments:               make([]diskCommitmentSecret, len(s.Commitments)),
	}
	for i, secret := range s.Commitments {
		disk.Commitments[i] = diskCommitmentSecret{
			CommitmentHeight: secret.CommitmentHeight,
			CommitmentRoot:   hex.EncodeToString(secret.CommitmentRoot),
			Left:             encodeLeaf(secret.Left),
			Right:            encodeLeaf(secret.Right),
		}
	}
	return json.MarshalIndent(disk, "", "  ")
}

func encodeLeaf(secret LeafSecret) diskLeafSecret {
	return diskLeafSecret{
		ProofHeight: secret.ProofHeight,
		Salt:        hex.EncodeToString(secret.Salt),
		LeafHash:    hex.EncodeToString(secret.LeafHash),
		Votes:       append([]verificationtypes.ProofVote(nil), secret.Votes...),
	}
}

func decodeState(data []byte) (State, error) {
	// Reject unknown fields and trailing JSON so schema mistakes cannot be
	// silently ignored during restart. Versioned strict decoding makes future
	// migrations explicit instead of accidentally accepting mixed formats.
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var disk diskState
	if err := decoder.Decode(&disk); err != nil {
		return State{}, fmt.Errorf("%w: %v", ErrInvalidLocalState, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return State{}, err
	}
	if len(disk.Commitments) > verificationtypes.MaxRevelationsPerBatch+1 {
		return State{}, fmt.Errorf("%w: too many commitment records", ErrInvalidLocalState)
	}
	operator, err := decodeHex("validator operator address", disk.ValidatorOperatorAddress)
	if err != nil {
		return State{}, err
	}
	keyHash, err := decodeHex("validator consensus key hash", disk.ValidatorConsensusKeyHash)
	if err != nil {
		return State{}, err
	}
	state := State{
		Version:                   disk.Version,
		ChainID:                   disk.ChainID,
		ValidatorOperatorAddress:  operator,
		ValidatorConsensusKeyHash: keyHash,
		Commitments:               make([]CommitmentSecret, len(disk.Commitments)),
	}
	for i, secret := range disk.Commitments {
		root, err := decodeHex("commitment root", secret.CommitmentRoot)
		if err != nil {
			return State{}, err
		}
		left, err := decodeLeaf(secret.Left)
		if err != nil {
			return State{}, err
		}
		right, err := decodeLeaf(secret.Right)
		if err != nil {
			return State{}, err
		}
		state.Commitments[i] = CommitmentSecret{
			CommitmentHeight: secret.CommitmentHeight,
			CommitmentRoot:   root,
			Left:             left,
			Right:            right,
		}
	}
	if err := ValidateState(state, false); err != nil {
		return State{}, err
	}
	return state, nil
}

func decodeLeaf(secret diskLeafSecret) (LeafSecret, error) {
	salt, err := decodeHex("leaf salt", secret.Salt)
	if err != nil {
		return LeafSecret{}, err
	}
	hash, err := decodeHex("leaf hash", secret.LeafHash)
	if err != nil {
		return LeafSecret{}, err
	}
	return LeafSecret{
		ProofHeight: secret.ProofHeight,
		Salt:        salt,
		LeafHash:    hash,
		Votes:       append([]verificationtypes.ProofVote(nil), secret.Votes...),
	}, nil
}

func decodeHex(name, value string) ([]byte, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid %s", ErrInvalidLocalState, name)
	}
	return decoded, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("%w: trailing JSON value", ErrInvalidLocalState)
		}
		return fmt.Errorf("%w: %v", ErrInvalidLocalState, err)
	}
	return nil
}
