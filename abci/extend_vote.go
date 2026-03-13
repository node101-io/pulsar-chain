package abci

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	voteextkeeper "github.com/node101-io/pulsar-chain/x/voteexthandler/keeper"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
	voteexthandler "github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

const (
	GenesisStateRoot = "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855"
)

type MinaSignatureVoteExt struct {
	MinaAddress string                     `json:"mina_address"`
	Signature   []byte                     `json:"signature"`
	VoteExtBody voteexthandler.VoteExtBody `json:"vote_ext_body"`
}

type VoteExtHandler struct {
	keyregistryKeeper keyregistrykeeper.Keeper
	voteextKeeper     voteextkeeper.Keeper

	MinaPrivateKey *types.SecondaryKey
	stakingKeeper  stakingkeeper.Keeper
	stateRoots     map[int64][]byte
	mu             sync.RWMutex
	votes          map[uint64]map[string][]byte // height -> consAddr -> extension bytes
}

func NewVoteExtHandler(keyregistryKeeper keyregistrykeeper.Keeper,
	voteextKeeper voteextkeeper.Keeper,
	minaPrivateKey *types.SecondaryKey) *VoteExtHandler {
	return &VoteExtHandler{
		keyregistryKeeper: keyregistryKeeper,
		voteextKeeper:     voteextKeeper,
		MinaPrivateKey:    minaPrivateKey,
		stateRoots:        make(map[int64][]byte),
		mu:                sync.RWMutex{},
		votes:             make(map[uint64]map[string][]byte),
	}
}

// ValidatorInfo represents a validator in the set
type ValidatorInfo struct {
	MinaAddress string
	Power       int64
}

func (h *VoteExtHandler) sortValidators(validators []ValidatorInfo) []ValidatorInfo {
	sort.Slice(validators, func(i, j int) bool {
		pubKeyI, err := new(keys.PublicKey).FromAddress(validators[i].MinaAddress)
		if err != nil {
			return false
		}
		pubKeyJ, err := new(keys.PublicKey).FromAddress(validators[j].MinaAddress)
		if err != nil {
			return false
		}
		return pubKeyI.X.Cmp(pubKeyJ.X) < 0
	})

	// Sondan başa gidiyoruz, parse edilebilen ilk index’i buluyoruz:
	lastValidIdx := len(validators) - 1
	for ; lastValidIdx >= 0; lastValidIdx-- {
		_, err := new(keys.PublicKey).FromAddress(validators[lastValidIdx].MinaAddress)
		if err == nil {
			break
		}
	}
	// Slice'ın sadece parse edilebilen validatorları içeren kısmını döndür
	return validators[:lastValidIdx+1]
}

// applyValidatorUpdates applies validator updates to the initial validator set
// and returns the new validator set sorted by address in ascending order
func (h *VoteExtHandler) applyValidatorUpdates(ctx sdk.Context, initialValidators []ValidatorInfo, updates []abci.ValidatorUpdate) ([]ValidatorInfo, error) {
	// Create a map for efficient lookups and updates
	validatorMap := make(map[string]*ValidatorInfo)

	// Add initial validators to the map
	for _, val := range initialValidators {
		key := string(val.MinaAddress)
		validatorMap[key] = &ValidatorInfo{
			MinaAddress: val.MinaAddress,
			Power:       val.Power,
		}
	}

	// Apply updates
	for _, update := range updates {
		// Convert public key to address
		pubKey, err := cryptocodec.FromCmtProtoPublicKey(update.GetPubKey())
		if err != nil {
			return nil, fmt.Errorf("failed to convert public key: %w", err)
		}

		// Use pubkey bytes as address (same logic as in initial validator set)
		consAddr := sdk.ConsAddress(pubKey.Address())

		exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, consAddr.Bytes())
		if !exists {
			return nil, fmt.Errorf("ExtendVoteHandler: failed to get key store for validator: %s", consAddr.String())
		}

		minaPubKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, consAddr.Bytes())
		if err != nil {
			return nil, fmt.Errorf("ExtendVoteHandler: failed to get key store for validator: %s", consAddr.String())
		}
		/*minaVal, found := h.Keeper.GetKeyStore(ctx, consAddr.String())
		if !found {
			return nil, fmt.Errorf("ExtendVoteHandler: failed to get key store for validator: %s", consAddr.String())
		}*/
		key := consAddr.String()

		if update.Power == 0 {
			// Remove validator if power is 0
			delete(validatorMap, key)
			ctx.Logger().Info("Removed validator from set", "consensus address", consAddr.String(), "mina address", minaPubKey)
		} else {
			// Add or update validator
			validatorMap[key] = &ValidatorInfo{
				MinaAddress: string(minaPubKey),
				Power:       update.Power,
			}
			ctx.Logger().Info("Added/Updated validator in set", "consensus address", consAddr.String(), "mina address", minaPubKey)
		}
	}

	// Convert map back to slice
	result := make([]ValidatorInfo, 0, len(validatorMap))
	for _, val := range validatorMap {
		result = append(result, *val)
	}

	result = h.sortValidators(result)

	return result, nil
}

// computeValidatorSetMerkleRoot computes the merkle root for a validator set
func (h *VoteExtHandler) computeValidatorSetMerkleRoot(validators []ValidatorInfo, poseidonHash *poseidon.Poseidon) (*big.Int, error) {
	input := []*big.Int{big.NewInt(0)}
	merkleRoot := poseidonHash.Hash(input)

	for _, validator := range validators {
		// Initialize the input array
		input = []*big.Int{}

		MinaPublicKey, err := keys.PublicKey{}.FromAddress(validator.MinaAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to convert validator address to public key: %w", err)
		}

		input = append(input, MinaPublicKey.X)
		if MinaPublicKey.IsOdd {
			input = append(input, big.NewInt(1))
		} else {
			input = append(input, big.NewInt(0))
		}

		// Hash the validator address
		hashOfAddr := poseidonHash.Hash(input)

		// Append the merkle root and the hash of the validator address to the input
		input = []*big.Int{merkleRoot, hashOfAddr}

		// Hash the input to get the new merkle root
		merkleRoot = poseidonHash.Hash(input)
	}

	return merkleRoot, nil
}

func (h *VoteExtHandler) ExtendVoteHandler() sdk.ExtendVoteHandler {
	return func(ctx sdk.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
		ctx.Logger().Info("ExtendVoteHandler", "height", req.GetHeight())

		h.stakingKeeper.GetBondedValidatorsByPower(ctx)
		// Get validator updates from the pending changes
		validatorUpdates, err := h.stakingKeeper.GetValidatorUpdates(ctx)
		if err != nil {

		}
		// Initialize poseidon hash
		poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)

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
				return nil, fmt.Errorf("ExtendVoteHandler: failed to get key store for validator: %s", consAddr.String())
			}

			minaPubKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, consAddr.Bytes())
			if err != nil {
				return nil, fmt.Errorf("ExtendVoteHandler: failed to get key store for validator: %s", consAddr.String())
			}

			/*minaVal, found := h.Keeper.GetKeyStore(ctx, consAddr.String())
			if !found {
				return nil, fmt.Errorf("ExtendVoteHandler: failed to get key store for validator: %s", consAddr.String())
			}*/

			initialValidators = append(initialValidators, ValidatorInfo{
				MinaAddress: string(minaPubKey),
				Power:       validator.GetConsensusPower(math.Int{}), // ??
			})
		}

		ctx.Logger().Info("Successfully got all cc validators", "ccValidators", initialValidators)

		initialValidators = h.sortValidators(initialValidators)

		initValSetRoot, err := h.computeValidatorSetMerkleRoot(initialValidators, poseidonHash)
		if err != nil {
			return nil, fmt.Errorf("failed to compute initial validator set root: %w", err)
		}
		ctx.Logger().Info("Successfully got initial validator set root", "initValSetRoot", initValSetRoot)

		prevStateRoot := h.stateRoots[req.GetHeight()-1]
		initStateRoot := h.stateRoots[req.GetHeight()]

		var extBody voteexthandler.VoteExtBody
		if len(validatorUpdates) != 0 {
			ctx.Logger().Info("Successfully got pending changes", "pendingChanges", validatorUpdates)

			// Apply validator set updates to the initial validator set and create merkle tree from the new validator set
			newValidatorSet, err := h.applyValidatorUpdates(ctx, initialValidators, validatorUpdates)
			if err != nil {
				return nil, fmt.Errorf("ExtendVoteHandler: failed to apply validator updates: %w", err)
			}

			newValSetRoot, err := h.computeValidatorSetMerkleRoot(newValidatorSet, poseidonHash)
			if err != nil {
				return nil, fmt.Errorf("failed to compute new validator set root: %w", err)
			}
			ctx.Logger().Info("Successfully computed new validator set root", "newValSetRoot", newValSetRoot, "validatorCount", len(newValidatorSet))

			// Construct the vote extension body
			extBody = voteexthandler.VoteExtBody{
				InitialValidatorSetRoot: initValSetRoot.Bytes(),
				InitialBlockHeight:      req.GetHeight() - 1,
				InitialStateRoot:        prevStateRoot,
				NewValidatorSetRoot:     newValSetRoot.Bytes(),
				NewBlockHeight:          req.GetHeight(),
				NewStateRoot:            initStateRoot,
			}
		} else {
			extBody = voteexthandler.VoteExtBody{
				InitialValidatorSetRoot: initValSetRoot.Bytes(),
				InitialBlockHeight:      req.GetHeight() - 1,
				InitialStateRoot:        prevStateRoot,
				NewValidatorSetRoot:     initValSetRoot.Bytes(),
				NewBlockHeight:          req.GetHeight(),
				NewStateRoot:            initStateRoot,
			}
		}

		// Hash the vote extension body
		extBodyHashInput := extBody.GetPoseidonHashInput(ctx, poseidonHash)

		// Sign the vote extension body

		signature, err := h.MinaPrivateKey.SecretKey.Sign(extBodyHashInput, types.DevnetNetworkID)
		ctx.Logger().Info("Signed block hash with secondary private key", "signature", signature)
		if err != nil {
			return nil, fmt.Errorf("failed to sign message: %w", err)
		}

		sigBytes, err := signature.MarshalBytes()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal signature: %w", err)
		}

		addr, err := h.MinaPrivateKey.PublicKey.ToAddress()
		if err != nil {
			return nil, fmt.Errorf("failed to convert public key to address: %w", err)
		}

		voteExt := MinaSignatureVoteExt{
			MinaAddress: addr,
			Signature:   sigBytes,
			VoteExtBody: extBody,
		}
		ctx.Logger().Info("Vote extension for block", "voteExt", voteExt)

		bz, err := json.Marshal(voteExt)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal vote extension: %w", err)
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

// storeVote saves the extension in-memory for later proposal processing.
func (h *VoteExtHandler) storeVote(height uint64, consAddr string, ext []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.votes == nil {
		h.votes = make(map[uint64]map[string][]byte)
	}
	if _, ok := h.votes[height]; !ok {
		h.votes[height] = make(map[string][]byte)
	}
	h.votes[height][consAddr] = ext
}

// fetchVotes returns a COPY of the map for the given height.
func (h *VoteExtHandler) fetchVotes(height uint64) map[string][]byte {
	h.mu.RLock()
	defer h.mu.RUnlock()
	res := make(map[string][]byte)
	if m, ok := h.votes[height]; ok {
		for k, v := range m {
			res[k] = v
		}
	}
	return res
}

// deleteVotes removes all saved votes for the given height.
func (h *VoteExtHandler) deleteVotes(height uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.votes, height)
}

func (h *VoteExtHandler) getVoteExtBody(height uint64) (types.VoteExtBody, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	addr, err := h.MinaPrivateKey.PublicKey.ToAddress()
	if err != nil {
		return types.VoteExtBody{}, fmt.Errorf("failed to convert node's own secondary public key to address: %w", err)
	}

	for _, vote := range h.votes[height] {
		var ve MinaSignatureVoteExt
		if ve.MinaAddress == addr {
			if err := json.Unmarshal(vote, &ve); err != nil {
				continue // skip malformed entry
			}
			return ve.VoteExtBody, nil
		}
	}
	return types.VoteExtBody{}, fmt.Errorf("vote extension not found for height %d", height)
}
