package types

const (
	EventTypeProofSubmitted       = "verification.proof_submitted"
	EventTypeCommitmentSubmitted  = "verification.commitment_submitted"
	EventTypeCommitmentRevealed   = "verification.commitment_revealed"
	EventTypeValidatorEquivocated = "verification.validator_equivocated"
	EventTypeProofFinalized       = "verification.proof_finalized"

	AttributeKeyProofHash        = "proof_hash"
	AttributeKeySubmissionHeight = "submission_height"
	AttributeKeyIndexInBlock     = "index_in_block"
	AttributeKeyProofType        = "proof_type"
	AttributeKeyValidator        = "validator"
	AttributeKeyCommitmentHeight = "commitment_height"
	AttributeKeyCommitment       = "commitment"
	AttributeKeyProofHeight      = "proof_height"
	AttributeKeyProofIndex       = "proof_index"
	AttributeKeyStatus           = "status"
	AttributeKeyTrueVotes        = "true_votes"
	AttributeKeyFalseVotes       = "false_votes"
	AttributeKeyValidatorCount   = "validator_count"
	AttributeKeyThreshold        = "threshold"
)
