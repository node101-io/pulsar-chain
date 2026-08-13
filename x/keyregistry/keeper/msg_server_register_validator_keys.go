package keeper

import (
	"context"

	"cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

// handleValidatorRegistration encapsulates the validator registration flow.
// The creator is the transaction signer/submitter; validator ownership is
// proven by the provided validator key and signatures.
func (k msgServer) handleValidatorRegistration(ctx context.Context, msg *types.MsgRegisterKeys) error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return errors.Wrap(types.ErrInvalidCreatorAddress, "creator address must be a valid bech32 address")
	}

	err = types.ValidatorPublicKeyPair{
		MinaKey:   msg.MinaPublicKey,
		CosmosKey: msg.CosmosPublicKey,
	}.Validate()
	if err != nil {
		return err
	}

	return k.persistValidatorRegistration(ctx, msg)
}

func (k msgServer) persistValidatorRegistration(ctx context.Context, msg *types.MsgRegisterKeys) error {
	cosmosKeyExists, err := k.Keeper.validatorCosmosToMina.Has(ctx, msg.CosmosPublicKey)
	if err != nil {
		return err
	}
	minaKeyExists, err := k.Keeper.validatorMinaToCosmos.Has(ctx, msg.MinaPublicKey)
	if err != nil {
		return err
	}
	if cosmosKeyExists || minaKeyExists {
		return errors.Wrap(types.ErrValidatorSecondaryKeyExists, "provided cosmos or mina public key is already registered")
	}

	challenge, err := types.RegistrationChallenge(msg.ActorType, msg.CosmosPublicKey)
	if err != nil {
		return err
	}

	minaSigValidity, err := VerifyMinaSig(msg.MinaSignature, challenge, msg.MinaPublicKey)
	if err != nil {
		return err
	}
	cosmosSigValidity := VerifyValidatorCosmosSig(msg.CosmosSignature, msg.MinaPublicKey, msg.CosmosPublicKey)
	if !minaSigValidity || !cosmosSigValidity {
		return errors.Wrap(types.ErrInvalidSignature, "invalid cosmos or mina signature")
	}

	err = k.Keeper.validatorCosmosToMina.Set(ctx, msg.CosmosPublicKey, msg.MinaPublicKey)
	if err != nil {
		return err
	}
	err = k.Keeper.validatorMinaToCosmos.Set(ctx, msg.MinaPublicKey, msg.CosmosPublicKey)
	if err != nil {
		return err
	}

	return nil
}
