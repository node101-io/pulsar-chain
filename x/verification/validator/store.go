package validator

type StateStore interface {
	Load() (State, error)
	Save(State) error
}
