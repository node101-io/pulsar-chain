package keeper

import (
	"bytes"
	"context"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"
	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

func (k msgServer) AddPublicKey(ctx context.Context, msg *types.MsgAddPublicKey) (*types.MsgAddPublicKeyResponse, error) {

	accountAddr, err := k.addressCodec.StringToBytes(msg.Creator)
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid creator address")
	}

	if msg.ProofHash == nil {
		return nil, types.ErrNilProofHash
	}

	if len(msg.ProofHash) != verificationtypes.ProofHashSize {
		return nil, types.ErrProofHashInvalidLength
	}

	if msg.PublicKeyInputs == nil {
		return nil, types.ErrNilPublicKeyInputs
	}
	if msg.PublicKeyInputs.Identity == nil {
		return nil, types.ErrNilIdentity
	}
	if len(msg.PublicKeyInputs.Identity) != types.IdentitySize {
		return nil, types.ErrIdentityInvalidLength
	}
	if msg.PublicKeyInputs.SessionPublicKey == nil {
		return nil, types.ErrNilPublicKey
	}

	if len(msg.PublicKeyInputs.SessionPublicKey) != types.SessionPublicKeySize {
		return nil, types.ErrPublicKeyInvalidLength
	}

	if msg.PublicKeyInputs.ExpiresAtHeight == 0 {
		return nil, types.ErrInvalidExpirationHeight
	}

	currentHeight := sdk.UnwrapSDKContext(ctx).BlockHeight()

	if msg.PublicKeyInputs.ExpiresAtHeight <= uint64(currentHeight) {
		return nil, types.ErrInvalidExpirationHeight
	}

	result, err := k.verificationKeeper.FinalProofResultByProofHash(
		ctx,
		msg.ProofHash,
	)
	if err != nil {
		return nil, err
	}

	if result.Status != verificationtypes.ProofStatus_PROOF_STATUS_VALID {
		return nil, types.ErrProofNotValid
	}

	params, err := k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(result.VerificationKeyHash,
		params.VerificationKeyHash) {
		return nil, types.ErrVerificationKeyMismatch
	}

	publicKeyInputsHash, err := types.ComputePublicInputsHash(msg.PublicKeyInputs)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(result.PublicInputsHash,
		publicKeyInputsHash) {
		return nil, types.ErrInvalidPublicInputsHash
	}

	if err := k.AppendSessionKeyToSmartAccount(ctx, msg.PublicKeyInputs.Identity, accountAddr, types.SessionKey{
		PublicKey:       msg.PublicKeyInputs.SessionPublicKey,
		ExpiresAtHeight: msg.PublicKeyInputs.ExpiresAtHeight,
	}); err != nil {
		return nil, err
	}

	return &types.MsgAddPublicKeyResponse{}, nil
}
