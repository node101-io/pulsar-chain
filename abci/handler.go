package abci

import (
	"context"
	"reflect"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	verificationvalidator "github.com/node101-io/pulsar-chain/x/verification/validator"
)

// VerificationPayloadBuilder supplies validator-local commitments and
// revelations for the composite vote extension. Every handler has a concrete
// builder, but that builder may return no work when verification is disabled or
// the sidecar has no terminal result. Consensus never consumes sidecar phases.
type VerificationPayloadBuilder interface {
	Build(context.Context, verificationvalidator.Identity, uint64) verificationvalidator.BuildOutcome
}

// ABCIHandler coordinates mandatory Mina transition signatures and non-blocking
// verification actions across ExtendVote, VerifyVoteExtension, proposal
// construction, proposal validation, and FinalizeBlock. Verification is always
// wired into ABCI, while a missing local result remains a slashable-duty concern
// for a future phase rather than a reason to stop block production.
type ABCIHandler struct {
	secondaryKey          SecondaryKey
	stakingKeeper         StakingKeeper
	keyregistryKeeper     KeyregistryKeeper
	votePersistenceKeeper VotePersistenceKeeper
	networkID             mina.NetworkID
	bridgeKeeper          BridgeKeeper
	verificationKeeper    VerificationKeeper
	verificationBuilder   VerificationPayloadBuilder
}

// NewABCIHandler validates every deterministic keeper and local producer needed
// by the composite vote-extension protocol. Disabled and full nodes use a
// concrete no-op builder instead of weakening the handler invariant with nil.
func NewABCIHandler(
	secondaryKey SecondaryKey,
	stakingKeeper StakingKeeper,
	keyregistryKeeper KeyregistryKeeper,
	votepersistenceKeeper VotePersistenceKeeper,
	networkId mina.NetworkID,
	bridgeKeeper BridgeKeeper,
	verificationKeeper VerificationKeeper,
	verificationBuilder VerificationPayloadBuilder,
) (*ABCIHandler, error) {
	if err := secondaryKey.Validate(); err != nil {
		return nil, err
	}
	if isNilDependency(stakingKeeper) {
		return nil, ErrMissingStakingKeeper
	}
	if isNilDependency(keyregistryKeeper) {
		return nil, ErrMissingKeyregistryKeeper
	}
	if isNilDependency(votepersistenceKeeper) {
		return nil, ErrMissingVotePersistenceKeeper
	}
	if isNilDependency(bridgeKeeper) {
		return nil, ErrMissingBridgeKeeper
	}
	if isNilDependency(verificationKeeper) {
		return nil, ErrMissingVerificationKeeper
	}
	if isNilDependency(verificationBuilder) {
		return nil, ErrMissingVerificationBuilder
	}
	return &ABCIHandler{
		secondaryKey:          secondaryKey,
		stakingKeeper:         stakingKeeper,
		keyregistryKeeper:     keyregistryKeeper,
		votePersistenceKeeper: votepersistenceKeeper,
		networkID:             networkId,
		bridgeKeeper:          bridgeKeeper,
		verificationKeeper:    verificationKeeper,
		verificationBuilder:   verificationBuilder,
	}, nil
}

func isNilDependency(value any) bool {
	if value == nil {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
