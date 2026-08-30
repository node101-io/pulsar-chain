package validator

import "sync"

// MemoryStore is a thread-safe StateStore for tests and embedded callers.
type MemoryStore struct {
	mu      sync.Mutex
	state   State
	loadErr error
	saveErr error
}

// NewMemoryStore creates a fresh unbound in-memory journal.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{state: EmptyState()}
}

// Load returns a defensive state copy.
func (s *MemoryStore) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return State{}, s.loadErr
	}
	return s.state.Clone(), nil
}

// Save replaces state with a defensive copy.
func (s *MemoryStore) Save(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return s.saveErr
	}
	s.state = state.Clone()
	return nil
}

// SetLoadError injects a deterministic test failure.
func (s *MemoryStore) SetLoadError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadErr = err
}

// SetSaveError injects a deterministic test failure.
func (s *MemoryStore) SetSaveError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveErr = err
}
