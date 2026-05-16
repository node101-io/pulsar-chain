package vote_ext

import (
	"errors"
	"math/big"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	votepersistencekeeper "github.com/node101-io/pulsar-chain/x/votepersistence/keeper"
	votepersistencetypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
	"github.com/stretchr/testify/require"
)

func TestExtendVoteHandlerDisabledHeight(t *testing.T) {
	ctx := sdk.Context{}.WithConsensusParams(tmproto.ConsensusParams{
		Abci: &tmproto.ABCIParams{VoteExtensionsEnableHeight: 1},
	})
	handler := (&AbciHandler{}).ExtendVoteHandler()

	response, err := handler(ctx, &abci.RequestExtendVote{Height: 3})

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Empty(t, response.VoteExtension)
}

func TestExtendVoteHandlerConsensusParamsFail(t *testing.T) {
	ctx := sdk.Context{}.WithConsensusParams(tmproto.ConsensusParams{})
	handler := (&AbciHandler{}).ExtendVoteHandler()

	response, err := handler(ctx, &abci.RequestExtendVote{Height: 4})

	require.Nil(t, response)
	require.ErrorIs(t, err, ErrUnableToReadConsensusParams)
}

func TestSignVoteExtBodyRoundTrip(t *testing.T) {
	secondaryKey := validSecondaryKey()
	body := votepersistencetypes.VoteExtBody{
		NextValidatorSetHash: []byte("next-validator-set-hash"),
		CurrentStateRoot:     []byte("current-state-root"),
		CurrentBlockHeight:   7,
		ActionsReducedRoot:   ActionsReducedRoot,
	}

	signature, err := secondaryKey.SignVoteExtBody(body)
	require.NoError(t, err)
	require.NotEmpty(t, signature)

	minaPublicKey, err := secondaryKey.PublicKey.MarshalBytes()
	require.NoError(t, err)

	poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)
	require.True(t, verifyVoteExtSig(poseidonHash, signature, body, minaPublicKey, ActionsReducedRoot))
}

func TestSecondaryKeyValidate(t *testing.T) {
	validKey := validSecondaryKey()
	require.NoError(t, validKey.Validate())

	require.ErrorIs(t, (SecondaryKey{}).Validate(), ErrMissingSecondaryKey)

	zeroPrivateKey := keys.PrivateKey{Value: big.NewInt(0)}
	zeroValueKey := SecondaryKey{
		SecretKey: &zeroPrivateKey,
		PublicKey: validKey.PublicKey,
	}
	require.ErrorIs(t, zeroValueKey.Validate(), ErrInvalidSecondaryKey)

	otherPrivateKey := keys.NewPrivateKeyFromBytes([32]byte{
		2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2,
	})
	otherPublicKey := otherPrivateKey.ToPublicKey()
	mismatchedKey := SecondaryKey{
		SecretKey: validKey.SecretKey,
		PublicKey: &otherPublicKey,
	}

	err := mismatchedKey.Validate()
	require.True(t, errors.Is(err, ErrInvalidSecondaryKey))
}

func TestNewABCIHandlerValidatesSecondaryKey(t *testing.T) {
	handler, err := NewABCIHandler(SecondaryKey{}, stakingkeeper.Keeper{}, keyregistrykeeper.Keeper{}, votepersistencekeeper.Keeper{})

	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrMissingSecondaryKey)
}

func validSecondaryKey() SecondaryKey {
	privateKey := keys.NewPrivateKeyFromBytes([32]byte{
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
	})
	publicKey := privateKey.ToPublicKey()

	return SecondaryKey{
		SecretKey: &privateKey,
		PublicKey: &publicKey,
	}
}
