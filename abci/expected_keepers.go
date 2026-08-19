package abci

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

type StakingKeeper interface {
	IterateLastValidators(context.Context, func(int64, stakingtypes.ValidatorI) bool) error
	GetHistoricalInfo(context.Context, int64) (stakingtypes.HistoricalInfo, error)
	GetValidatorByConsAddr(context.Context, sdk.ConsAddress) (stakingtypes.Validator, error)
}

type KeyregistryKeeper interface {
	ValidatorCosmosToMinaHas(context.Context, []byte) (bool, error)
	ValidatorGetCosmosToMina(context.Context, []byte) ([]byte, error)
}

type VotePersistenceKeeper interface {
	Clear(context.Context) error
	SetVote(context.Context, int64, []byte, []byte) error
}

type BridgeKeeper interface {
	GetActionsReducedRootAtHeight(context.Context, int64) ([]byte, error)
}

// VerificationKeeper is the deterministic, replicated state surface used by
// ABCI. It knows registered proofs, snapshots, commitments, and revelations,
// but never calls the local sidecar. Sidecar results and private salts enter
// through the separate validator builder so machine-local failures cannot make
// keeper execution nondeterministic.
type VerificationKeeper interface {
	ValidateVerificationPayload(context.Context, []byte, uint64, []byte, []verificationtypes.CommitmentRevelation) error
	ApplyVerificationPayload(context.Context, []byte, uint64, []byte, []verificationtypes.CommitmentRevelation) error
	CreateValidatorSnapshot(context.Context, uint64) error
	GetProofsAtHeight(context.Context, uint64) ([]verificationtypes.ProofEntry, error)
	GetCommitment(context.Context, []byte, uint64) ([]byte, bool, error)
}
