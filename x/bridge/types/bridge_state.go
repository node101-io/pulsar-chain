package types

import (
	"math/big"
	"strings"

	errorsmod "cosmossdk.io/errors"
	minafield "github.com/node101-io/mina-signer-go/field"
)

// Validate checks BridgeState height, batch-shape, and canonical valid_action_hashes invariants.
func (s BridgeState) Validate() error {
	if s.LatestFetchedMinaHeight < 0 {
		return ErrInvalidLatestFetchedMinaHeight
	}
	if s.ValidActionHashesCosmosBlockHeight < 0 {
		return ErrInvalidBridgeStateHeight
	}
	if s.StartMinaHeight < 0 {
		return ErrInvalidBridgeStateHeight
	}
	if s.StartMinaHeight > s.LatestFetchedMinaHeight {
		return errorsmod.Wrapf(
			ErrInvalidBridgeStateHeight,
			"start_mina_height %d must be <= latest_fetched_mina_height %d",
			s.StartMinaHeight,
			s.LatestFetchedMinaHeight,
		)
	}
	if s.ValidActionHashesCosmosBlockHeight == 0 {
		if len(s.ValidActionHashes) != 0 || s.StartMinaHeight != s.LatestFetchedMinaHeight {
			return errorsmod.Wrap(
				ErrInvalidValidActionBatch,
				"initial batch must be empty and start_mina_height must equal latest_fetched_mina_height",
			)
		}
	} else if s.StartMinaHeight >= s.LatestFetchedMinaHeight {
		return errorsmod.Wrap(
			ErrInvalidValidActionBatch,
			"runtime batch must advance the Mina cursor",
		)
	}

	for i, hash := range s.ValidActionHashes {
		if err := validateCanonicalActionHash(hash); err != nil {
			return errorsmod.Wrapf(err, "valid_action_hashes[%d] must be a canonical Mina field element decimal", i)
		}
	}

	return nil
}

func validateCanonicalActionHash(hash string) error {
	if strings.TrimSpace(hash) != hash || hash == "" {
		return ErrInvalidValidActionHash
	}

	n, ok := new(big.Int).SetString(hash, 10)
	if !ok || n.Sign() < 0 || n.String() != hash {
		return ErrInvalidValidActionHash
	}

	field := minafield.NewField()
	raw := n.Bytes()
	if len(raw) > field.ElementSize() {
		return ErrInvalidValidActionHash
	}

	fixed := make([]byte, field.ElementSize())
	copy(fixed[len(fixed)-len(raw):], raw)

	element, err := field.FromBytes(fixed)
	if err != nil || element.String() != hash {
		return ErrInvalidValidActionHash
	}

	return nil
}
