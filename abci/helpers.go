package vote_ext

import (
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/node101-io/mina-signer-go/keys"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
)

var VoteExtMarker []byte = []byte("VOTEEXT:")

const AcceptanceRatio float64 = 0.66

type SecondaryKey struct {
	SecretKey *keys.PrivateKey
	PublicKey *keys.PublicKey
}

type AbciHandler struct {
	secondaryKey      SecondaryKey
	stakingKeeper     stakingkeeper.Keeper
	keyregistryKeeper keyregistrykeeper.Keeper
}

type VoteExtensionBody struct {
	NextValidatorSetHash []byte
	CurrentStateRoot     []byte
	NextBlockHeight      int64
}

type payload struct {
	Height uint64            `json:"height"`
	Votes  map[string][]byte `json:"votes"`
}

func NewVoteExtHandler(secondaryKey SecondaryKey, stakingKeeper stakingkeeper.Keeper, keyregistryKeeper keyregistrykeeper.Keeper) *AbciHandler {
	return &AbciHandler{
		secondaryKey:      secondaryKey,
		stakingKeeper:     stakingKeeper,
		keyregistryKeeper: keyregistryKeeper,
	}
}
