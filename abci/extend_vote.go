package vote_ext

import (
	"encoding/json"

	"cosmossdk.io/errors"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
	voteexthandler "github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

var hardcoded = [32]byte{
	0x7a, 0x13, 0x9f, 0x42, 0xd1, 0x8c, 0x5e, 0xa7,
	0x2b, 0x6d, 0xf0, 0x91, 0x3c, 0x47, 0xb8, 0x0e,
	0x55, 0x1a, 0xcd, 0x72, 0x98, 0x04, 0xe6, 0xaf,
	0x39, 0xb2, 0x7c, 0x5d, 0x11, 0x8e, 0xf3, 0x64,
}

// ValidatorInfo represents a validator in the set
type ValidatorInfo struct {
	MinaAddress string
	Power       int64
}

func (h *VoteExtHandler) wrapValidatorInfo(ctx sdk.Context) ([]ValidatorInfo, error) {
	var initialValidators []ValidatorInfo
	var callbackErr error

	err := h.stakingKeeper.IterateLastValidators(ctx, func(index int64, validator stakingtypes.ValidatorI) (stop bool) {
		consAddr, err := validator.ConsPubKey()
		if err != nil {
			callbackErr = errors.Wrap(types.ErrInternal, err.Error())
			return true
		}

		exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, consAddr.Bytes())
		if err != nil {
			callbackErr = errors.Wrap(types.ErrInternal, err.Error())
			return true
		}
		if !exists {
			callbackErr = errors.Wrap(types.ErrFailedToGetKeystore, "")
			return true
		}

		minaPubKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, consAddr.Bytes())
		if err != nil {
			callbackErr = errors.Wrap(types.ErrInternal, err.Error())
			return true
		}

		initialValidators = append(initialValidators, ValidatorInfo{
			MinaAddress: string(minaPubKey),
			Power:       validator.GetConsensusPower(h.stakingKeeper.PowerReduction(ctx)),
		})
		return false
	})
	if err != nil {
		return nil, err
	}
	if callbackErr != nil {
		return nil, callbackErr
	}

	initialValidators = h.sortValidators(initialValidators)
	return initialValidators, nil
}

// TODO: Update this method when switching to consumer chain
func (h *VoteExtHandler) constructMinaSignatureVoteExt(extBody voteexthandler.Body, ctx sdk.Context, hash *poseidon.Poseidon) (MinaSignatureVoteExt, error) {

	// Hash the vote extension body
	extBodyHashInput := extBody.GetPoseidonHashInput(ctx, hash)

	// Sign the vote extension body
	signature, err := h.MinaPrivateKey.SecretKey.Sign(extBodyHashInput, types.DevnetNetworkID)
	if err != nil {
		return MinaSignatureVoteExt{}, errors.Wrap(types.ErrFailedToSign, err.Error())
	}

	sigBytes, err := signature.MarshalBytes()
	if err != nil {
		return MinaSignatureVoteExt{}, errors.Wrap(types.ErrFailedToMarshal, err.Error())
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

	return voteExt, nil
}

func (h *VoteExtHandler) constructVoteExtBody(ctx sdk.Context, req *abci.RequestExtendVote, hash poseidon.Poseidon) (voteexthandler.Body, error) {

	/*validatorUpdates, err := h.stakingKeeper.GetValidatorUpdates(ctx)
	if err != nil {
		return voteexthandler.Body{}, err
	}

	initialValidators, err := h.wrapValidatorInfo(ctx)
	if err != nil {
		return voteexthandler.Body{}, err
	}

	initValSetRoot, err := h.computeValidatorSetMerkleRoot(initialValidators, &hash)
	if err != nil {
		return voteexthandler.Body{}, errors.Wrap(types.ErrFailedToComputeInitialValidatorSetRoot, err.Error())
	}*/

	prevStateRoot := h.stateRoots[req.GetHeight()-1]
	initStateRoot := h.stateRoots[req.GetHeight()]

	var extBody voteexthandler.Body
	/*if len(validatorUpdates) != 0 {

		// Apply validator set updates to the initial validator set and create merkle tree from the new validator set
		newValidatorSet, err := h.applyValidatorUpdates(ctx, initialValidators, validatorUpdates)
		if err != nil {
			return voteexthandler.Body{}, errors.Wrap(types.ErrFailedToApplyValidatorUpdates, "ExtendVoteHandler"+err.Error())
		}

		newValSetRoot, err := h.computeValidatorSetMerkleRoot(newValidatorSet, &hash)
		if err != nil {
			return voteexthandler.Body{}, errors.Wrap(types.ErrFailedToComputeNewSetRoot, err.Error())
		}

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
	}*/

	extBody = voteexthandler.Body{
		InitialValidatorSetRoot: hardcoded[:],
		InitialBlockHeight:      req.GetHeight() - 1,
		InitialStateRoot:        prevStateRoot,
		NewValidatorSetRoot:     hardcoded[:],
		NewBlockHeight:          req.GetHeight(),
		NewStateRoot:            initStateRoot,
	}

	return extBody, nil
}

func (h *VoteExtHandler) ExtendVoteHandler() sdk.ExtendVoteHandler {
	return func(ctx sdk.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {

		ctx.Logger().Info("extend vote handler called")
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
		ctx.Logger().Info("vote ext body extend vote handler", voteExt.VoteExtBody.NewBlockHeight)
		// Store vote extension in memory
		h.storeVote(uint64(req.GetHeight()), voteExt.MinaAddress, bz)

		return &abci.ResponseExtendVote{VoteExtension: bz}, nil
	}
}
