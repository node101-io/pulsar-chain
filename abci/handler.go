package abci

import (
	"context"
	"reflect"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	verificationvalidator "github.com/node101-io/pulsar-chain/x/verification/validator"
)

// VerificationPayloadBuilder supplies optional validator-local commitments and
// revelations for the composite vote extension. It intentionally exposes no
// sidecar phase or error state: consensus only needs terminal proof results and
// the on-chain actions derived from them.
type VerificationPayloadBuilder interface {
	Build(context.Context, verificationvalidator.Identity, uint64) verificationvalidator.BuildOutcome
}

// ABCIHandler coordinates mandatory Mina transition signatures and optional
// verification actions across ExtendVote, VerifyVoteExtension, proposal
// construction, proposal validation, and FinalizeBlock. This is the boundary
// where validator-local computation becomes authenticated consensus input.
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

// NewABCIHandler validates mandatory dependencies. Verification dependencies
// may be nil because proof verification is an additive, best-effort producer of
// vote-extension data; disabling it must not disable the existing consensus and
// Mina-signature path.
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
