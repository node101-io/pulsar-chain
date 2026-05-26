package keeper

import (
	"context"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/publickey"
	"github.com/node101-io/mina-signer-go/signature"
	"github.com/node101-io/pulsar-chain/x/keyregistry/types"
)

func VerifyMinaSig(sig []byte, msg, minaAddress []byte, actorType types.ActorType) (bool, error) {

	minaPk, err := publickey.NewPublicKeyFromBytes(minaAddress, mina.NetworkID(actorType.String()))
	if err != nil {
		return false, err
	}

	minaSig, err := signature.NewSignatureFromBytes(sig)
	if err != nil {
		return false, err
	}

	return minaPk.VerifyBytes(minaSig, msg)
}

func VerifyUserCosmosSig(sig []byte, msg, cosmosAddress []byte) bool {

	cosmosPk := secp256k1.PubKey{
		Key: cosmosAddress,
	}

	return cosmosPk.VerifySignature(msg, sig)
}

func VerifyValidatorCosmosSig(sig []byte, msg, cosmosAddress []byte) bool {

	var cosmosValidatorPubKey ed25519.PubKey = cosmosAddress

	return cosmosValidatorPubKey.VerifySignature(msg, sig)
}

// deriveAddressFromPubkey derives the expected signer address from the provided
// key material.
func deriveAddressFromPubkey(actorType types.ActorType, cosmosPublicKey []byte) (string, error) {

	if actorType != types.ActorType_USER {
		return "", types.ErrInvalidActorType
	}

	pubKey := secp256k1.PubKey{
		Key: cosmosPublicKey,
	}

	addr := sdk.AccAddress(pubKey.Address())
	return addr.String(), nil

}

// RegisterKeys registers a Mina and Cosmos public key pair on chain.
// It verifies that:
//   - the creator address is valid
//   - the cosmos public key is a valid compressed secp256k1 key (33 bytes)
//   - the creator address matches the provided cosmos public key
//   - neither the cosmos nor mina public key is already registered
//   - both the mina and cosmos signatures are valid
//
// If all checks pass, the key pair is stored in both the CosmosToMina and MinaToCosmos maps.
func (k msgServer) RegisterKeys(ctx context.Context, msg *types.MsgRegisterKeys) (*types.MsgRegisterKeysResponse, error) {

	var err error

	switch msg.ActorType {
	case types.ActorType_USER:
		err = k.handleUserRegistration(ctx, msg)
	case types.ActorType_VALIDATOR:
		err = k.handleValidatorRegistration(ctx, msg)
	default:
		return nil, types.ErrInvalidActorType
	}
	if err != nil {
		return nil, err
	}

	return &types.MsgRegisterKeysResponse{}, nil
}
