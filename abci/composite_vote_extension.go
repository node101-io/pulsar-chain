package abci

import (
	"bytes"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
	verificationvalidator "github.com/node101-io/pulsar-chain/x/verification/validator"
)

const (
	CompositeVoteExtensionVersion     uint32 = 1
	MaxCompositeVoteExtensionBytes           = 16 << 10
	DefaultVerificationSidecarTimeout        = 100 * time.Millisecond
)

func encodeCompositeVoteExtension(extension *CompositeVoteExtension) ([]byte, error) {
	if extension == nil {
		return nil, ErrInvalidCompositeVoteExtension
	}
	encoded, err := extension.Marshal()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCompositeVoteExtension, err)
	}
	if len(encoded) == 0 || len(encoded) > MaxCompositeVoteExtensionBytes {
		return nil, ErrInvalidCompositeVoteExtension
	}

	return encoded, nil
}

func decodeCompositeVoteExtension(encoded []byte) (*CompositeVoteExtension, error) {
	if len(encoded) == 0 || len(encoded) > MaxCompositeVoteExtensionBytes {
		return nil, ErrInvalidCompositeVoteExtension
	}
	var extension CompositeVoteExtension
	if err := extension.Unmarshal(encoded); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCompositeVoteExtension, err)
	}
	canonical, err := extension.Marshal()
	if err != nil || !bytes.Equal(canonical, encoded) {
		return nil, ErrInvalidCompositeVoteExtension
	}
	if extension.ProtocolVersion != CompositeVoteExtensionVersion || len(extension.TransitionSignature) == 0 {
		return nil, ErrInvalidCompositeVoteExtension
	}

	return &extension, nil
}

func validateVerificationPayloadStructure(payload *verificationtypes.VerificationVoteExtensionPayload, targetHeight uint64) error {
	if payload == nil {
		return nil
	}
	if payload.TargetHeight != targetHeight {
		return fmt.Errorf("%w: target height", ErrInvalidVerificationPayload)
	}
	if len(payload.Commitment) != 0 && len(payload.Commitment) != verificationtypes.CommitmentHashSize {
		return verificationtypes.ErrInvalidCommitmentLength
	}
	if len(payload.Revelations) > verificationtypes.MaxRevelationsPerBatch {
		return verificationtypes.ErrInvalidRevelationCount
	}
	if len(payload.Commitment) == 0 && len(payload.Revelations) == 0 {
		return ErrInvalidVerificationPayload
	}

	seenHeights := make(map[uint64]struct{}, len(payload.Revelations))
	for _, revelation := range payload.Revelations {
		if _, exists := seenHeights[revelation.CommitmentHeight]; exists {
			return verificationtypes.ErrDuplicateCommitmentRevelation
		}
		seenHeights[revelation.CommitmentHeight] = struct{}{}
		if err := verificationtypes.ValidateRevelationCommitmentHeight(targetHeight, revelation.CommitmentHeight); err != nil {
			return err
		}
		leftHeight, rightHeight, err := verificationtypes.CommitmentProofHeights(revelation.CommitmentHeight)
		if err != nil {
			return err
		}
		leftHashOnly, err := validateVerificationLeaf(payload.TargetHeight, leftHeight, revelation.Left)
		if err != nil {
			return err
		}
		rightHashOnly, err := validateVerificationLeaf(payload.TargetHeight, rightHeight, revelation.Right)
		if err != nil {
			return err
		}
		if leftHashOnly && rightHashOnly {
			return verificationtypes.ErrUselessRevelation
		}
	}

	return nil
}

func validateVerificationLeaf(currentHeight, proofHeight uint64, leaf verificationtypes.LeafRevelation) (bool, error) {
	switch leaf.Mode {
	case verificationtypes.LeafRevealMode_LEAF_REVEAL_MODE_HASH_ONLY:
		if len(leaf.GetLeafHash()) != verificationtypes.LeafHashSize {
			return true, verificationtypes.ErrInvalidLeafHashLength
		}
		return true, nil
	case verificationtypes.LeafRevealMode_LEAF_REVEAL_MODE_VALUE:
		if verificationtypes.GetLeafTiming(currentHeight, proofHeight) == verificationtypes.LeafTooEarly {
			return false, verificationtypes.ErrEarlyReveal
		}
		value := leaf.GetValue()
		if value == nil {
			return false, verificationtypes.ErrInvalidLeafRevealMode
		}
		if len(value.Salt) != verificationtypes.SaltSize {
			return false, verificationtypes.ErrInvalidSaltLength
		}
		if err := verificationtypes.ValidateCanonicalVotes(value.Votes); err != nil {
			return false, err
		}
		return false, nil
	default:
		return false, verificationtypes.ErrInvalidLeafRevealMode
	}
}

func (h *ABCIHandler) buildVerificationPayload(ctx sdk.Context, sourceHeight int64) *verificationtypes.VerificationVoteExtensionPayload {
	if h.verificationKeeper == nil || h.verificationBuilder == nil {
		return nil
	}
	targetHeight, err := verificationvalidator.TargetHeight(sourceHeight)
	if err != nil {
		return nil
	}
	identity, err := h.localVerificationIdentity(ctx, sourceHeight)
	if err != nil {
		ctx.Logger().Error("verification payload omitted", "reason", err)
		return nil
	}
	outcome := h.verificationBuilder.Build(ctx, identity, targetHeight)
	if outcome.Warning != nil {
		ctx.Logger().Error("verification payload partially or fully omitted", "reason", outcome.Warning)
	}
	if outcome.Payload == nil || validateVerificationPayloadStructure(outcome.Payload, targetHeight) != nil {
		return nil
	}
	return outcome.Payload
}

func (h *ABCIHandler) localVerificationIdentity(
	ctx sdk.Context,
	sourceHeight int64,
) (verificationvalidator.Identity, error) {
	if h.secondaryKey.PublicKey == nil {
		return verificationvalidator.Identity{}, ErrMissingSecondaryKey
	}
	validators, err := h.getValidatorSet(ctx, sourceHeight)
	if err != nil {
		return verificationvalidator.Identity{}, err
	}
	for _, current := range validators {
		publicKey, err := current.ConsPubKey()
		if err != nil {
			return verificationvalidator.Identity{}, err
		}
		minaPublicKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, publicKey.Bytes())
		if err != nil {
			return verificationvalidator.Identity{}, err
		}
		if !bytes.Equal(minaPublicKey, h.secondaryKey.PublicKey.Bytes()) {
			continue
		}
		operator, err := sdk.ValAddressFromBech32(current.GetOperator())
		if err != nil {
			return verificationvalidator.Identity{}, err
		}
		return verificationvalidator.Identity{
			ChainID: ctx.ChainID(), OperatorAddress: operator, ConsensusPublicKey: append([]byte(nil), publicKey.Bytes()...),
		}, nil
	}
	return verificationvalidator.Identity{}, fmt.Errorf("local validator is not in validator set at height %d", sourceHeight)
}
