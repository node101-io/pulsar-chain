package vote_ext

import (
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/node101-io/mina-signer-go/keys"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	votepersistence "github.com/node101-io/pulsar-chain/x/votepersistence/keeper"
)

type SecondaryKey struct {
	SecretKey *keys.PrivateKey
	PublicKey *keys.PublicKey
}

type AbciHandler struct {
	secondaryKey          SecondaryKey
	stakingKeeper         stakingkeeper.Keeper
	keyregistryKeeper     keyregistrykeeper.Keeper
	votePersistenceKeeper votepersistence.Keeper
}

func NewABCIHandler(secondaryKey SecondaryKey, stakingKeeper stakingkeeper.Keeper,
	keyregistryKeeper keyregistrykeeper.Keeper, votepersistenceKeeper votepersistence.Keeper) *AbciHandler {
	return &AbciHandler{
		secondaryKey:          secondaryKey,
		stakingKeeper:         stakingKeeper,
		keyregistryKeeper:     keyregistryKeeper,
		votePersistenceKeeper: votepersistenceKeeper,
	}
}
