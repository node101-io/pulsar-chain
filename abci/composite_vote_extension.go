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
	// CompositeVoteExtensionVersion identifies the outer wire envelope that
	// CometBFT signs. Versioning lets future nodes reject incompatible layouts
	// instead of interpreting the same signed bytes under different rules.
	CompositeVoteExtensionVersion uint32 = 1
	// MaxCompositeVoteExtensionBytes bounds gossip, signature verification, and
	// proposal work caused by one validator. Verification is optional, so it must
	// not be able to make the mandatory consensus path unbounded.
	MaxCompositeVoteExtensionBytes = 16 << 10
	// DefaultVerificationSidecarTimeout limits how long ExtendVote waits for the
	// local sidecar. Missing this optional result is safer than delaying consensus;
	// the overlapping H+2/H+3 commitment windows provide another opportunity.
	DefaultVerificationSidecarTimeout = 100 * time.Millisecond
)

// A composite vote extension carries two related but independently checked
// pieces of data. TransitionSignature is the existing mandatory Mina signature.
// VerificationPayload is an optional commitment or revelation assembled from
// local sidecar results. CometBFT then signs the complete encoded envelope, so
// another validator cannot copy or alter the optional payload while keeping the
// original validator's consensus identity.

// encodeCompositeVoteExtension serializes the versioned envelope and enforces
// the consensus-wide byte limit before CometBFT signs it. Checking the limit at
// creation and decoding keeps honest and remote paths on the same resource bound.
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

// decodeCompositeVoteExtension rejects oversized, non-canonical, unsupported,
// or Mina-signature-free envelopes. The Mina component remains mandatory even
// when no verifier is configured, preserving the chain's pre-verification vote
// extension contract.
func decodeCompositeVoteExtension(encoded []byte) (*CompositeVoteExtension, error) {
	if len(encoded) == 0 || len(encoded) > MaxCompositeVoteExtensionBytes {
		return nil, ErrInvalidCompositeVoteExtension
	}
	var extension CompositeVoteExtension
	if err := extension.Unmarshal(encoded); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCompositeVoteExtension, err)
	}
	// Re-marshalling prevents different protobuf byte encodings of the same
	// logical payload from being accepted as equivalent signed messages.
	canonical, err := extension.Marshal()
	if err != nil || !bytes.Equal(canonical, encoded) {
		return nil, ErrInvalidCompositeVoteExtension
	}
	if extension.ProtocolVersion != CompositeVoteExtensionVersion || len(extension.TransitionSignature) == 0 {
		return nil, ErrInvalidCompositeVoteExtension
	}

	return &extension, nil
}

// validateVerificationPayloadStructure performs cheap context-free checks in
// every ABCI phase before keeper state validation. Repeating these checks at
// construction, vote verification, and proposal validation keeps malformed
// optional data from crossing a trust boundary, while the keeper remains the
// authority for state-dependent timing, eligibility, and duplicate rules.
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

// buildVerificationPayload asks the always-present local builder for actions
// that a proposal at sourceHeight+1 can apply. A disabled builder or unavailable
// sidecar returns no new work; this records a missed validator opportunity
// without changing the mandatory transition signature or treating proofs as invalid.
func (h *ABCIHandler) buildVerificationPayload(ctx sdk.Context, sourceHeight int64) *verificationtypes.VerificationVoteExtensionPayload {
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

// localVerificationIdentity resolves the local Mina signer back to the
// historical validator operator and consensus key used at sourceHeight. The
// local journal binds commitment preimages to this full identity so copied node
// data, a chain-ID change, or validator key rotation cannot reveal a commitment
// under the wrong validator identity.
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
