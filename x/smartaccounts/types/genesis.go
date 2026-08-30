package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:        DefaultParams(),
		SmartAccounts: []SmartAccountEntry{},
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}

	identities := make(map[string]struct{}, len(gs.SmartAccounts))
	for i, entry := range gs.SmartAccounts {
		if len(entry.Identity) != IdentitySize {
			return fmt.Errorf(
				"smart account %d identity must be %d bytes: got %d",
				i,
				IdentitySize,
				len(entry.Identity),
			)
		}

		identity := string(entry.Identity)
		if _, exists := identities[identity]; exists {
			return fmt.Errorf("duplicate smart account identity at index %d", i)
		}
		identities[identity] = struct{}{}

		if err := sdk.VerifyAddressFormat(entry.Account.AccountAddress); err != nil {
			return fmt.Errorf("smart account %d has invalid account address: %w", i, err)
		}

		sessionKeys := make(map[string]map[uint64]struct{}, len(entry.Account.SessionKeys))
		for j, key := range entry.Account.SessionKeys {
			if len(key.PublicKey) != SessionPublicKeySize {
				return fmt.Errorf(
					"smart account %d session key %d must be %d bytes: got %d",
					i,
					j,
					SessionPublicKeySize,
					len(key.PublicKey),
				)
			}
			if key.ExpiresAtHeight == 0 {
				return fmt.Errorf("smart account %d session key %d has zero expiration height", i, j)
			}

			publicKey := string(key.PublicKey)
			expirations, exists := sessionKeys[publicKey]
			if !exists {
				expirations = make(map[uint64]struct{})
				sessionKeys[publicKey] = expirations
			}
			if _, exists := expirations[key.ExpiresAtHeight]; exists {
				return fmt.Errorf("duplicate session key at smart account %d index %d", i, j)
			}
			expirations[key.ExpiresAtHeight] = struct{}{}
		}
	}

	return nil
}
