package types

func (s BridgeState) Validate() error {
	if s.LatestFetchedMinaHeight < 0 {
		return ErrInvalidLatestFetchedMinaHeight
	}

	return validateActionsReducedRoot(s.CurrentActionsReducedRoot)
}
