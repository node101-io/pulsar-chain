package vote_ext

import (
	"encoding/json"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
	voteexthandler "github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

// ValidatorInfo represents a validator in the set
type ValidatorInfo struct {
	MinaAddress string
	Power       int64
}

func (h *VoteExtHandler) wrapValidatorInfo(ctx sdk.Context) ([]ValidatorInfo, error) {
	// Get all Cross-chain validators
	validators, err := h.stakingKeeper.GetAllValidators(ctx)
	if err != nil {
		return nil, err
	}
	// Convert CCValidators to ValidatorInfo format
	initialValidators := make([]ValidatorInfo, 0, len(validators))
	for _, validator := range validators {
		consAddr := sdk.ConsAddress(validator.OperatorAddress)

		exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, consAddr.Bytes())
		if !exists {
			return nil, errors.Wrap(types.ErrFailedToGetKeystore, consAddr.String())
		}

		minaPubKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, consAddr.Bytes())
		if err != nil {
			return nil, errors.Wrap(types.ErrInternal, err.Error())
		}

		initialValidators = append(initialValidators, ValidatorInfo{
			MinaAddress: string(minaPubKey),
			Power:       validator.GetConsensusPower(math.Int{}), // ??
		})
	}

	ctx.Logger().Info("Successfully got all cc validators", "ccValidators", initialValidators)

	initialValidators = h.sortValidators(initialValidators)

	return initialValidators, nil
}

func (h *VoteExtHandler) constructMinaSignatureVoteExt(extBody voteexthandler.Body, ctx sdk.Context, hash *poseidon.Poseidon) (MinaSignatureVoteExt, error) {

	// Hash the vote extension body
	extBodyHashInput := extBody.GetPoseidonHashInput(ctx, hash)

	// Sign the vote extension body
	signature, err := h.MinaPrivateKey.SecretKey.Sign(extBodyHashInput, types.DevnetNetworkID)
	ctx.Logger().Info("Signed block hash with secondary private key", "signature", signature)
	if err != nil {
		return MinaSignatureVoteExt{}, errors.Wrap(types.ErrFailedToSign, err.Error())
	}

	sigBytes, err := signature.MarshalBytes()
	if err != nil {
		return MinaSignatureVoteExt{}, errors.Wrap(types.ErrFailedToMarshal, "signature")
	}

	addr, err := h.MinaPrivateKey.PublicKey.ToAddress()
	if err != nil {
		return MinaSignatureVoteExt{}, errors.Wrap(types.ErrFailedToConvertPubKeyToAddr, err.Error())
	}

	voteExt := MinaSignatureVoteExt{
		MinaAddress: addr,
		Signature:   sigBytes,
		VoteExtBody: extBody,
	}
	ctx.Logger().Info("Vote extension for block", "voteExt", voteExt)

	return MinaSignatureVoteExt{}, nil
}

func (h *VoteExtHandler) constructVoteExtBody(ctx sdk.Context, req *abci.RequestExtendVote, hash poseidon.Poseidon) (voteexthandler.Body, error) {

	// Get validator updates from the pending changes
	validatorUpdates, err := h.stakingKeeper.GetValidatorUpdates(ctx)
	if err != nil {
		return voteexthandler.Body{}, err
	}

	initialValidators, err := h.wrapValidatorInfo(ctx)
	if err != nil {
		return voteexthandler.Body{}, err
	}
	ctx.Logger().Info("Successfully got all cc validators", "ccValidators", initialValidators)

	initValSetRoot, err := h.computeValidatorSetMerkleRoot(initialValidators, &hash)
	if err != nil {
		return voteexthandler.Body{}, errors.Wrap(types.ErrFailedToComputeInitialValidatorSetRoot, err.Error())
	}
	ctx.Logger().Info("Successfully got initial validator set root", "initValSetRoot", initValSetRoot)

	prevStateRoot := h.stateRoots[req.GetHeight()-1]
	initStateRoot := h.stateRoots[req.GetHeight()]

	var extBody voteexthandler.Body
	if len(validatorUpdates) != 0 {
		ctx.Logger().Info("Successfully got pending changes", "pendingChanges", validatorUpdates)

		// Apply validator set updates to the initial validator set and create merkle tree from the new validator set
		newValidatorSet, err := h.applyValidatorUpdates(ctx, initialValidators, validatorUpdates)
		if err != nil {
			return voteexthandler.Body{}, errors.Wrap(types.ErrFailedToApplyValidatorUpdates, "ExtendVoteHandler"+err.Error())
		}

		newValSetRoot, err := h.computeValidatorSetMerkleRoot(newValidatorSet, &hash)
		if err != nil {
			return voteexthandler.Body{}, errors.Wrap(types.ErrFailedToComputeNewSetRoot, err.Error())
		}
		ctx.Logger().Info("Successfully computed new validator set root", "newValSetRoot", newValSetRoot, "validatorCount", len(newValidatorSet))

		// Construct the vote extension body
		extBody = voteexthandler.Body{
			InitialValidatorSetRoot: initValSetRoot.Bytes(),
			InitialBlockHeight:      req.GetHeight() - 1,
			InitialStateRoot:        prevStateRoot,
			NewValidatorSetRoot:     newValSetRoot.Bytes(),
			NewBlockHeight:          req.GetHeight(),
			NewStateRoot:            initStateRoot,
		}
	} else {
		extBody = voteexthandler.Body{
			InitialValidatorSetRoot: initValSetRoot.Bytes(),
			InitialBlockHeight:      req.GetHeight() - 1,
			InitialStateRoot:        prevStateRoot,
			NewValidatorSetRoot:     initValSetRoot.Bytes(),
			NewBlockHeight:          req.GetHeight(),
			NewStateRoot:            initStateRoot,
		}
	}
	return extBody, err
}

func (h *VoteExtHandler) ExtendVoteHandler() sdk.ExtendVoteHandler {
	return func(ctx sdk.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
		ctx.Logger().Info("ExtendVoteHandler", "height", req.GetHeight())

		// Initialize poseidon hash
		poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

		voteExtBody, err := h.constructVoteExtBody(ctx, req, *poseidonHash)
		if err != nil {
			return nil, err
		}
		voteExt, err := h.constructMinaSignatureVoteExt(voteExtBody, ctx, poseidonHash)
		if err != nil {
			return nil, err
		}
		bz, err := json.Marshal(voteExt)
		if err != nil {
			return nil, errors.Wrap(types.ErrFailedToMarshal, err.Error())
		}

		// Store vote extension in memory
		h.storeVote(uint64(req.GetHeight()), voteExt.MinaAddress, bz)
		ctx.Logger().Info("Vote extension stored in memory", "height", req.GetHeight(), "validator", voteExt.MinaAddress)
		votes := h.fetchVotes(uint64(req.GetHeight()))
		// Log votes with height and validator address
		ctx.Logger().Info("Votes", "height", req.GetHeight(), "votes", votes)

		return &abci.ResponseExtendVote{VoteExtension: bz}, nil
	}
}
