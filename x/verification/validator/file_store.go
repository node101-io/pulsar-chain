package validator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cometbft/cometbft/libs/tempfile"
)

const (
	stateFileMode          = 0o600
	stateDirMode           = 0o700
	maxLocalStateFileBytes = 256 << 10
)

type FileStore struct {
	path string
}

func NewFileStore(path string) (*FileStore, error) {
	if path == "" || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("%w: local state path must be absolute", ErrInvalidLocalState)
	}
	return &FileStore{path: filepath.Clean(path)}, nil
}

func (s *FileStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *FileStore) Load() (State, error) {
	if s == nil || s.path == "" {
		return State{}, ErrInvalidLocalState
	}
	info, err := os.Lstat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return EmptyState(), nil
	}
	if err != nil {
		return State{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return State{}, fmt.Errorf("%w: local state path is not a regular file", ErrInvalidLocalState)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return State{}, fmt.Errorf("%w: local state file permissions must be 0600", ErrInvalidLocalState)
	}
	if info.Size() > maxLocalStateFileBytes {
		return State{}, fmt.Errorf("%w: local state file exceeds %d bytes", ErrInvalidLocalState, maxLocalStateFileBytes)
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return State{}, err
	}
	return decodeState(data)
}

func (s *FileStore) Save(state State) error {
	if s == nil || s.path == "" {
		return ErrInvalidLocalState
	}
	data, err := encodeState(state)
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := ensureStateDirectory(dir); err != nil {
		return err
	}
	if info, err := os.Lstat(s.path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("%w: local state path is not a regular file", ErrInvalidLocalState)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := tempfile.WriteFileAtomic(s.path, data, stateFileMode); err != nil {
		return err
	}
	return syncDirectory(dir)
}

func ensureStateDirectory(dir string) error {
	if err := os.MkdirAll(dir, stateDirMode); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%w: local state directory is not a directory", ErrInvalidLocalState)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%w: local state directory permissions must not allow group or other access", ErrInvalidLocalState)
	}
	return nil
}

func syncDirectory(dir string) error {
	file, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}
