package abci

import (
	"sync"

	verificationTypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

type revealStore struct {
	reveal map[int64]verificationTypes.ProofCommitmentReveal
	mu     sync.Mutex
}

func NewRevealStore() *revealStore {
	return &revealStore{
		reveal: make(map[int64]verificationTypes.ProofCommitmentReveal),
	}
}

func (s *revealStore) Set(blockHeight int64,
	comm verificationTypes.ProofCommitmentReveal) {

	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.reveal[blockHeight] = comm
}

func (s *revealStore) Has(blockHeight int64) bool {

	if s == nil {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.reveal[blockHeight]

	return exists
}

func (s *revealStore) Get(blockHeight int64) (verificationTypes.ProofCommitmentReveal, error) {

	if s == nil {
		return verificationTypes.ProofCommitmentReveal{}, ErrRevealNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	comm, exists := s.reveal[blockHeight]
	if !exists {
		return verificationTypes.ProofCommitmentReveal{}, ErrRevealNotFound
	}

	return comm, nil
}
