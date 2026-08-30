package validator

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

func TestFileStoreRoundTripAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "state.json")
	store, err := NewFileStore(path)
	require.NoError(t, err)
	state := validTestState(t)
	require.NoError(t, store.Save(state))

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	loaded, err := store.Load()
	require.NoError(t, err)
	require.Equal(t, state, loaded)
}

func TestFileStoreRejectsCorruptionUnknownFieldsAndSymlink(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(dir, 0o700))

	corruptPath := filepath.Join(dir, "corrupt.json")
	require.NoError(t, os.WriteFile(corruptPath, []byte(`{"version":1,"unknown":true}`), 0o600))
	corrupt, err := NewFileStore(corruptPath)
	require.NoError(t, err)
	_, err = corrupt.Load()
	require.ErrorIs(t, err, ErrInvalidLocalState)

	target := filepath.Join(dir, "target.json")
	require.NoError(t, os.WriteFile(target, []byte("{}"), 0o600))
	symlinkPath := filepath.Join(dir, "state.json")
	require.NoError(t, os.Symlink(target, symlinkPath))
	symlink, err := NewFileStore(symlinkPath)
	require.NoError(t, err)
	_, err = symlink.Load()
	require.ErrorIs(t, err, ErrInvalidLocalState)
}

func TestFileStoreRejectsLoosePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(dir, 0o700))
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o644))
	store, err := NewFileStore(path)
	require.NoError(t, err)
	_, err = store.Load()
	require.ErrorIs(t, err, ErrInvalidLocalState)
}

func TestFileStoreRejectsOversizedState(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(dir, 0o700))
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte{'x'}, maxLocalStateFileBytes+1), 0o600))
	store, err := NewFileStore(path)
	require.NoError(t, err)
	_, err = store.Load()
	require.ErrorIs(t, err, ErrInvalidLocalState)
}

func validTestState(t testing.TB) State {
	t.Helper()
	leftSalt := bytes.Repeat([]byte{1}, verificationtypes.SaltSize)
	rightSalt := bytes.Repeat([]byte{2}, verificationtypes.SaltSize)
	leftVotes := []verificationtypes.ProofVote{{IndexInBlock: 0, Result: true}}
	rightVotes := []verificationtypes.ProofVote{{IndexInBlock: 1, Result: false}}
	leftHash, err := verificationtypes.ComputeLeafHash(leftSalt, leftVotes)
	require.NoError(t, err)
	rightHash, err := verificationtypes.ComputeLeafHash(rightSalt, rightVotes)
	require.NoError(t, err)
	root := verificationtypes.ComputeCommitmentRoot(leftHash, rightHash)
	return State{
		Version: StateVersion, ChainID: "chain", ValidatorOperatorAddress: bytes.Repeat([]byte{3}, 20),
		ValidatorConsensusKeyHash: bytes.Repeat([]byte{4}, 32),
		Commitments: []CommitmentSecret{{
			CommitmentHeight: 10, CommitmentRoot: root[:],
			Left:  LeafSecret{ProofHeight: 7, Salt: leftSalt, LeafHash: leftHash[:], Votes: leftVotes},
			Right: LeafSecret{ProofHeight: 8, Salt: rightSalt, LeafHash: rightHash[:], Votes: rightVotes},
		}},
	}
}
