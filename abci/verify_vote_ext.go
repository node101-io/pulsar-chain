package abci

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"

	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/poseidon"
	keyregistryTypes "github.com/node101-io/pulsar-chain/x/keyregistry/types"
	verificationTypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

func (h *ABCIHandler) VerifyVoteExtensionHandler() sdk.VerifyVoteExtensionHandler {
	return func(ctx sdk.Context, req *cometabci.RequestVerifyVoteExtension) (*cometabci.ResponseVerifyVoteExtension, error) {

		shouldVerifyVoteExtension, err := shouldExtendVoteAtHeight(ctx, req.GetHeight())
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}

		if !shouldVerifyVoteExtension {
			if len(req.VoteExtension) == 0 {
				return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_ACCEPT}, nil
			}

			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, nil
		}

		voteExtension, err := decodeVoteExtension(req.VoteExtension)
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, nil
		}

		cosmosValidatorPubKey, err := h.getConsPubKeyByConsAddr(ctx, req.ValidatorAddress)
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}

		exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, cosmosValidatorPubKey)
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}

		if !exists {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, nil
		}

		minaKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, cosmosValidatorPubKey)
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}

		body, err := h.constructVoteExtBody(ctx, req.GetHeight())
		if err != nil {
			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}
		poseidonHash := poseidon.NewPoseidon()

		if err := verifyVoteExtSig(
			poseidonHash,
			voteExtension.Signature,
			body,
			voteExtension.ProofCommitment,
			minaKey,
			h.networkID,
		); err != nil {
			if errors.Is(err, keyregistryTypes.ErrValidatorNotRegistered) ||
				errors.Is(err, ErrInvalidVoteExtSignatureEncoding) ||
				errors.Is(err, ErrInvalidVoteExtSignature) {
				return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, nil
			}

			return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_REJECT}, err
		}

		return &cometabci.ResponseVerifyVoteExtension{Status: cometabci.ResponseVerifyVoteExtension_ACCEPT}, nil
	}
}

func verifyReveal(reveal *verificationTypes.ProofCommitmentReveal,
	previousCommitment []byte) error {

	firstInput := append([]byte{}, reveal.FirstSecretSalt...)
	firstInput = append(firstInput, encodeLength(len(reveal.FirstBlockProofs))...)

	for i := range reveal.FirstBlockProofs {
		bz, err := reveal.FirstBlockProofs[i].Marshal()
		if err != nil {
			return err
		}
		firstInput = append(firstInput, bz...)
	}

	firstHash := sha256.Sum256(firstInput)
	firstLeaf := firstHash[:16]

	finalInput := append([]byte{}, firstLeaf...)
	finalInput = append(finalInput, reveal.SecondLeafHash...)

	finalHash := sha256.Sum256(finalInput)
	reconstructed := finalHash[:16]

	if !bytes.Equal(reconstructed, previousCommitment) {
		return fmt.Errorf("")
	}

	return nil
}

func (h *ABCIHandler) getPreviousProofCommitment(
	ctx sdk.Context,
	currentVoteExtensionHeight int64,
	minaKey []byte,
) ([]byte, error) {
	// Current extension H, revealStore[H-1]'de üretilmiş
	// commitment'ı açıyor.
	//
	// Vote persistence şu anda extension height yerine
	// signedStateHeight = extensionHeight - 2 ile key'liyor.
	//
	// Previous extension: H-1
	// Persistence key: (H-1)-2 = H-3
	previousStorageHeight := currentVoteExtensionHeight - 3

	bz, err := h.votePersistenceKeeper.GetVote(
		ctx,
		previousStorageHeight,
		minaKey,
	)
	if err != nil {
		return nil, fmt.Errorf("get previous vote extension: %w", err)
	}

	previousVoteExtension, err := decodeVoteExtension(bz)
	if err != nil {
		return nil, fmt.Errorf("decode previous vote extension: %w", err)
	}

	if !validateProofCommitment(previousVoteExtension.ProofCommitment) {
		return nil, ErrInvalidProofCommitment
	}

	return previousVoteExtension.ProofCommitment, nil
}
