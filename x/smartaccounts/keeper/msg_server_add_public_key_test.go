package keeper_test

import (
	"bytes"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/node101-io/pulsar-chain/x/smartaccounts/keeper"
	"github.com/node101-io/pulsar-chain/x/smartaccounts/types"
	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

func TestAddPublicKey(t *testing.T) {
	f := initFixture(t)
	msg := validAddPublicKeyMessage(t, f)
	server := keeper.NewMsgServerImpl(f.keeper)

	response, err := server.AddPublicKey(f.ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, msg.VerificationId, f.verificationKeeper.requestedVerificationID)

	exists, err := f.keeper.HasSmartAccount(f.ctx, msg.PublicKeyInputs.Identity)
	require.NoError(t, err)
	require.True(t, exists)

	accountAddress, err := f.addressCodec.StringToBytes(msg.Creator)
	require.NoError(t, err)
	genesis, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.Len(t, genesis.SmartAccounts, 1)
	require.Equal(t, accountAddress, genesis.SmartAccounts[0].Account.AccountAddress)

	_, err = server.AddPublicKey(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrSessionKeyAlreadyExists)
}

func TestAddPublicKeyRejectsCreatorNotBoundByProof(t *testing.T) {
	f := initFixture(t)
	msg := validAddPublicKeyMessage(t, f)
	server := keeper.NewMsgServerImpl(f.keeper)

	otherCreator, err := f.addressCodec.BytesToString(bytes.Repeat([]byte{0x09}, 20))
	require.NoError(t, err)
	msg.Creator = otherCreator

	_, err = server.AddPublicKey(f.ctx, msg)
	require.ErrorIs(t, err, types.ErrInvalidPublicInputsHash)

	originalCreator, err := f.addressCodec.BytesToString(f.keeper.GetAuthority())
	require.NoError(t, err)
	msg.Creator = originalCreator

	_, err = server.AddPublicKey(f.ctx, msg)
	require.NoError(t, err)

	genesis, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.Len(t, genesis.SmartAccounts, 1)
	require.Equal(t, f.keeper.GetAuthority(), genesis.SmartAccounts[0].Account.AccountAddress)
}

func TestAddPublicKeyRejectsInvalidProofData(t *testing.T) {
	f := initFixture(t)
	msg := validAddPublicKeyMessage(t, f)
	validResult := f.verificationKeeper.result

	invalidResult := validResult
	invalidResult.Status = verificationtypes.ProofStatus_PROOF_STATUS_INVALID

	verificationKeyMismatch := validResult
	verificationKeyMismatch.VerificationKeyHash = bytes.Clone(validResult.VerificationKeyHash)
	verificationKeyMismatch.VerificationKeyHash[0] ^= 0xff

	publicInputsHashMismatch := validResult
	publicInputsHashMismatch.PublicInputsHash = bytes.Clone(validResult.PublicInputsHash)
	publicInputsHashMismatch.PublicInputsHash[0] ^= 0xff

	tests := []struct {
		name            string
		result          verificationtypes.FinalProofResult
		verificationErr error
		want            error
	}{
		{
			name:            "proof not found",
			result:          validResult,
			verificationErr: verificationtypes.ErrProofNotFound,
			want:            verificationtypes.ErrProofNotFound,
		},
		{
			name:   "proof is invalid",
			result: invalidResult,
			want:   types.ErrProofNotValid,
		},
		{
			name:   "verification key mismatch",
			result: verificationKeyMismatch,
			want:   types.ErrVerificationKeyMismatch,
		},
		{
			name:   "public inputs hash mismatch",
			result: publicInputsHashMismatch,
			want:   types.ErrInvalidPublicInputsHash,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f.verificationKeeper.result = test.result
			f.verificationKeeper.err = test.verificationErr

			_, err := keeper.NewMsgServerImpl(f.keeper).AddPublicKey(f.ctx, msg)
			require.ErrorIs(t, err, test.want)
		})
	}
}

func TestAddPublicKeyRejectsInvalidInputs(t *testing.T) {
	f := initFixture(t)
	validMsg := validAddPublicKeyMessage(t, f)

	tests := []struct {
		name            string
		verificationID  []byte
		publicKeyInputs *types.PublicKeyInputs
		want            error
	}{
		{
			name: "nil verification ID",
			publicKeyInputs: &types.PublicKeyInputs{
				SessionPublicKey: validMsg.PublicKeyInputs.SessionPublicKey,
				ExpiresAtHeight:  validMsg.PublicKeyInputs.ExpiresAtHeight,
				Identity:         validMsg.PublicKeyInputs.Identity,
			},
			want: types.ErrNilVerificationID,
		},
		{
			name:           "invalid verification ID length",
			verificationID: validMsg.VerificationId[:verificationtypes.VerificationIDSize-1],
			publicKeyInputs: &types.PublicKeyInputs{
				SessionPublicKey: validMsg.PublicKeyInputs.SessionPublicKey,
				ExpiresAtHeight:  validMsg.PublicKeyInputs.ExpiresAtHeight,
				Identity:         validMsg.PublicKeyInputs.Identity,
			},
			want: types.ErrVerificationIDInvalidLength,
		},
		{
			name:           "nil public key inputs",
			verificationID: validMsg.VerificationId,
			want:           types.ErrNilPublicKeyInputs,
		},
		{
			name:           "nil identity",
			verificationID: validMsg.VerificationId,
			publicKeyInputs: &types.PublicKeyInputs{
				SessionPublicKey: validMsg.PublicKeyInputs.SessionPublicKey,
				ExpiresAtHeight:  validMsg.PublicKeyInputs.ExpiresAtHeight,
			},
			want: types.ErrNilIdentity,
		},
		{
			name:           "invalid identity length",
			verificationID: validMsg.VerificationId,
			publicKeyInputs: &types.PublicKeyInputs{
				SessionPublicKey: validMsg.PublicKeyInputs.SessionPublicKey,
				ExpiresAtHeight:  validMsg.PublicKeyInputs.ExpiresAtHeight,
				Identity:         validMsg.PublicKeyInputs.Identity[:types.IdentitySize-1],
			},
			want: types.ErrIdentityInvalidLength,
		},
		{
			name:           "nil public key",
			verificationID: validMsg.VerificationId,
			publicKeyInputs: &types.PublicKeyInputs{
				ExpiresAtHeight: validMsg.PublicKeyInputs.ExpiresAtHeight,
				Identity:        validMsg.PublicKeyInputs.Identity,
			},
			want: types.ErrNilPublicKey,
		},
		{
			name:           "invalid public key length",
			verificationID: validMsg.VerificationId,
			publicKeyInputs: &types.PublicKeyInputs{
				SessionPublicKey: validMsg.PublicKeyInputs.SessionPublicKey[:types.SessionPublicKeySize-1],
				ExpiresAtHeight:  validMsg.PublicKeyInputs.ExpiresAtHeight,
				Identity:         validMsg.PublicKeyInputs.Identity,
			},
			want: types.ErrPublicKeyInvalidLength,
		},
		{
			name:           "zero expiration height",
			verificationID: validMsg.VerificationId,
			publicKeyInputs: &types.PublicKeyInputs{
				SessionPublicKey: validMsg.PublicKeyInputs.SessionPublicKey,
				ExpiresAtHeight:  0,
				Identity:         validMsg.PublicKeyInputs.Identity,
			},
			want: types.ErrInvalidExpirationHeight,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			msg := &types.MsgAddPublicKey{
				Creator:         validMsg.Creator,
				VerificationId:  test.verificationID,
				PublicKeyInputs: test.publicKeyInputs,
			}

			_, err := keeper.NewMsgServerImpl(f.keeper).AddPublicKey(f.ctx, msg)
			require.ErrorIs(t, err, test.want)
		})
	}
}

func TestAddPublicKeyRejectsExpiredSessionKey(t *testing.T) {
	f := initFixture(t)
	msg := validAddPublicKeyMessage(t, f)

	sdkCtx := sdk.UnwrapSDKContext(f.ctx).WithBlockHeight(int64(msg.PublicKeyInputs.ExpiresAtHeight))

	_, err := keeper.NewMsgServerImpl(f.keeper).AddPublicKey(sdkCtx, msg)
	require.ErrorIs(t, err, types.ErrInvalidExpirationHeight)
}

func validAddPublicKeyMessage(t *testing.T, f *fixture) *types.MsgAddPublicKey {
	t.Helper()

	creator, err := f.addressCodec.BytesToString(f.keeper.GetAuthority())
	require.NoError(t, err)
	accountAddress, err := f.addressCodec.StringToBytes(creator)
	require.NoError(t, err)

	currentHeight := sdk.UnwrapSDKContext(f.ctx).BlockHeight()
	inputs := &types.PublicKeyInputs{
		SessionPublicKey: bytes.Repeat([]byte{0x02}, types.SessionPublicKeySize),
		ExpiresAtHeight:  uint64(currentHeight) + 10,
		Identity:         bytes.Repeat([]byte{0x03}, types.IdentitySize),
	}
	publicInputsHash, err := types.ComputePublicInputsHash(inputs, accountAddress)
	require.NoError(t, err)

	proofHash := bytes.Repeat([]byte{0x04}, verificationtypes.ProofHashSize)
	verificationKeyHash := bytes.Repeat([]byte{0x01}, types.VerificationKeyHashSize)
	verificationID, err := verificationtypes.ComputeVerificationID(
		verificationtypes.ProofType_PROOF_TYPE_NOIR_BARRETENBERG,
		proofHash,
		publicInputsHash,
		verificationKeyHash,
	)
	require.NoError(t, err)
	f.verificationKeeper.result = verificationtypes.FinalProofResult{
		ProofHash:           bytes.Clone(proofHash),
		ProofType:           verificationtypes.ProofType_PROOF_TYPE_NOIR_BARRETENBERG,
		Status:              verificationtypes.ProofStatus_PROOF_STATUS_VALID,
		PublicInputsHash:    publicInputsHash,
		VerificationKeyHash: verificationKeyHash,
		VerificationId:      bytes.Clone(verificationID[:]),
	}

	return &types.MsgAddPublicKey{
		Creator:         creator,
		VerificationId:  verificationID[:],
		PublicKeyInputs: inputs,
	}
}
