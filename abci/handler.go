package abci

import (
	"reflect"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
)

type ABCIHandler struct {
	secondaryKey          SecondaryKey
	stakingKeeper         StakingKeeper
	keyregistryKeeper     KeyregistryKeeper
	votePersistenceKeeper VotePersistenceKeeper
	networkID             mina.NetworkID
	bridgeKeeper          BridgeKeeper
	verificationKeeper    VerificationKeeper
	revealStore           *revealStore
}

func NewABCIHandler(
	secondaryKey SecondaryKey,
	stakingKeeper StakingKeeper,
	keyregistryKeeper KeyregistryKeeper,
	votepersistenceKeeper VotePersistenceKeeper,
	networkId mina.NetworkID,
	bridgeKeeper BridgeKeeper,
	verificationKeeper VerificationKeeper,
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

	return &ABCIHandler{
		secondaryKey:          secondaryKey,
		stakingKeeper:         stakingKeeper,
		keyregistryKeeper:     keyregistryKeeper,
		votePersistenceKeeper: votepersistenceKeeper,
		networkID:             networkId,
		bridgeKeeper:          bridgeKeeper,
		verificationKeeper:    verificationKeeper,
		revealStore:           NewRevealStore(),
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
