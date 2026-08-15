package types

import "cosmossdk.io/collections"

const (
	// ModuleName defines the module name
	ModuleName = "keyregistry"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// GovModuleName duplicates the gov module's name to avoid a dependency with x/gov.
	// It should be synced with the gov module's name if it is ever changed.
	// See: https://github.com/cosmos/cosmos-sdk/blob/v0.52.0-beta.2/x/gov/types/keys.go#L9
	GovModuleName = "gov"
)

// ParamsKey is the prefix to retrieve all Params
var ParamsKey = collections.NewPrefix("p_keyregistry")
var UserCosmosToMinaPrefix = collections.NewPrefix("user_cosmos_map")
var UserMinaToCosmosPrefix = collections.NewPrefix("user_mina_map")
var UserKeyVersionPrefix = collections.NewPrefix("user_key_version")

var ValidatorCosmosToMinaPrefix = collections.NewPrefix("validator_cosmos_map")
var ValidatorMinaToCosmosPrefix = collections.NewPrefix("validator_mina_map")
var ValidatorKeyVersionPrefix = collections.NewPrefix("validator_key_version")
