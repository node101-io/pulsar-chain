package validator

import "sync"

type MemoryStore struct {
	mu      sync.Mutex
	state   State
	loadErr error
	saveErr error
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{state: EmptyState()}
}

func (s *MemoryStore) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return State{}, s.loadErr
	}
	return s.state.Clone(), nil
}

func (s *MemoryStore) Save(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return s.saveErr
	}
	s.state = state.Clone()
	return nil
}

func (s *MemoryStore) SetLoadError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadErr = err
}

func (s *MemoryStore) SetSaveError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveErr = err
}
