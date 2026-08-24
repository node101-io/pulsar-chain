package keeper

import (
	"bytes"
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"
)

const SmartAccountsMapName = "smart_accounts_map"

type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// Address capable of executing a MsgUpdateParams message.
	// Typically, this should be the x/gov module account.
	authority []byte

	smartAccounts collections.Map[[]byte, types.SmartAccount] // account_id --> session keys

	verificationKeeper types.VerificationKeeper

	Schema collections.Schema
	Params collections.Item[types.Params]
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	verificationKeeper types.VerificationKeeper,
) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}

	sb := collections.NewSchemaBuilder(storeService)

	k := Keeper{
		storeService: storeService,
		cdc:          cdc,
		addressCodec: addressCodec,
		authority:    authority,

		Params: collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),

		smartAccounts: collections.NewMap(
			sb,
			types.SmartAccountsKey,
			SmartAccountsMapName,
			collections.BytesKey,
			codec.CollValue[types.SmartAccount](cdc)),

		verificationKeeper: verificationKeeper,
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema

	return k
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() []byte {
	return k.authority
}

func (k Keeper) HasSmartAccount(ctx context.Context, identity []byte) (bool, error) {

	if identity == nil {
		return false, types.ErrNilIdentity
	}

	if len(identity) == 0 {
		return false, types.ErrNilIdentity
	}

	if len(identity) != types.IdentitySize {
		return false, types.ErrIdentityInvalidLength
	}

	return k.smartAccounts.Has(ctx, identity)
}

func (k Keeper) AppendSessionKeyToSmartAccount(ctx context.Context, identity []byte, key types.SessionKey) error {

	if identity == nil {
		return types.ErrNilIdentity
	}

	if len(identity) == 0 {
		return types.ErrNilIdentity
	}

	if len(identity) != types.IdentitySize {
		return types.ErrIdentityInvalidLength
	}

	if key.PublicKey == nil {
		return types.ErrNilPublicKey
	}

	if len(key.PublicKey) != types.SessionPublicKeySize {
		return types.ErrPublicKeyInvalidLength
	}

	if key.ExpiresAtHeight == 0 {
		return types.ErrInvalidExpirationHeight
	}

	exists, err := k.HasSmartAccount(ctx, identity)
	if err != nil {
		return err
	}

	if !exists {
		err := k.smartAccounts.Set(ctx, identity, types.SmartAccount{
			SessionKeys: []types.SessionKey{
				key,
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	acc, err := k.smartAccounts.Get(ctx, identity)
	if err != nil {
		return err
	}

	for _, existingKey := range acc.SessionKeys {
		if bytes.Equal(existingKey.PublicKey, key.PublicKey) &&
			existingKey.ExpiresAtHeight == key.ExpiresAtHeight {
			return types.ErrSessionKeyAlreadyExists
		}
	}

	acc.SessionKeys = append(acc.SessionKeys, key)

	if err := k.smartAccounts.Set(ctx, identity, types.SmartAccount{
		SessionKeys: acc.SessionKeys,
	}); err != nil {
		return err
	}

	return k.pruneSessionKey(ctx, identity)
}

func (k Keeper) pruneSessionKey(ctx context.Context, identity []byte) error {

	var validSessionKeys []types.SessionKey

	smartAcc, err := k.smartAccounts.Get(ctx, identity)
	if err != nil {
		return err
	}

	currentBlockHeight := uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())

	for _, key := range smartAcc.SessionKeys {

		if currentBlockHeight >= key.ExpiresAtHeight {
			continue
		}
		validSessionKeys = append(validSessionKeys, key)
	}

	if err := k.smartAccounts.Set(ctx, identity, types.SmartAccount{
		SessionKeys: validSessionKeys,
	}); err != nil {
		return err
	}

	return nil
}
