package validator

// StateStore persists validator-local commitment preimages across restarts.
type StateStore interface {
	Load() (State, error)
	Save(State) error
}
