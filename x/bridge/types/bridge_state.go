package types

import (
	"strings"

	errorsmod "cosmossdk.io/errors"
)

func (s BridgeState) Validate() error {
	if s.LatestFetchedMinaHeight < 0 {
		return ErrInvalidLatestFetchedMinaHeight
	}

	for i, hash := range s.ValidActionHashes {
		trimmed := strings.TrimSpace(hash)
		if trimmed == "" {
			return errorsmod.Wrapf(ErrInvalidValidActionHash, "valid_action_hashes[%d] must not be empty", i)
		}
		if trimmed != hash {
			return errorsmod.Wrapf(ErrInvalidValidActionHash, "valid_action_hashes[%d] must not contain surrounding whitespace", i)
		}
	}

	return nil
}
