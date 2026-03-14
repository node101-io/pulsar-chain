package vote_ext

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"cosmossdk.io/errors"
	abci "github.com/cometbft/cometbft/abci/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/mina-signer-go/signature"
	keyregistrykeeper "github.com/node101-io/pulsar-chain/x/keyregistry/keeper"
	voteextkeeper "github.com/node101-io/pulsar-chain/x/voteexthandler/keeper"
	"github.com/node101-io/pulsar-chain/x/voteexthandler/types"
	voteexthandler "github.com/node101-io/pulsar-chain/x/voteexthandler/types"
)

type MinaSignatureVoteExt struct {
	MinaAddress string              `json:"mina_address"`
	Signature   []byte              `json:"signature"`
	VoteExtBody voteexthandler.Body `json:"vote_ext_body"`
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
func GetSecondaryKeys(appOpts servertypes.AppOptions) types.SecondaryKey {
	minaPrivKey := appOpts.Get("vote_extension.priv_key")
	keyStr, ok := minaPrivKey.(string)
	if !ok {
		panic("vote_extension.priv_key is not a string")
	}

	// Decode base64 -> bytes
	keyBytes, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		panic(fmt.Sprintf("failed to decode base64 priv key: %v", err))
	}

	// Bytes -> big.Int
	prv := new(big.Int).SetBytes(keyBytes)
	priv := keys.PrivateKey{
		Value: prv,
	}
	public := priv.ToPublicKey()
	secondaryKey := types.SecondaryKey{
		SecretKey: &priv,
		PublicKey: &public,
	}

	return secondaryKey
}

func verifySchnorr(voteExt MinaSignatureVoteExt, pubKey keys.PublicKey, ctx sdk.Context, hash poseidon.Poseidon) error {
	extBodyHashInput := voteExt.VoteExtBody.GetPoseidonHashInput(ctx, &hash)
	sig := new(signature.Signature)
	if err := sig.UnmarshalBytes(voteExt.Signature); err != nil {
		return errors.Wrap(types.ErrInvalidSigEncoding, "")
	}
	// Verify signature; if ok, keep the vote in memory.
	if !pubKey.Verify(sig, extBodyHashInput, types.DevnetNetworkID) {
		return errors.Wrap(types.ErrInvalidSignature, "")
	}
	return nil
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
			return nil, errors.Wrap(types.ErrFailedToConvertPubKeyToAddr, err.Error())
		}
		// Use pubkey bytes as address (same logic as in initial validator set)
		consAddr := sdk.ConsAddress(pubKey.Address())

		exists, err := h.keyregistryKeeper.ValidatorCosmosToMinaHas(ctx, consAddr.Bytes())
		if err != nil {
			return nil, errors.Wrap(types.ErrFailedToGetKeystore, err.Error())
		}
		if !exists {
			return nil, errors.Wrap(types.ErrUnknownValidator, "")
		}
		minaPubKey, err := h.keyregistryKeeper.ValidatorGetCosmosToMina(ctx, consAddr.Bytes())
		if err != nil {
			return nil, errors.Wrap(types.ErrInternal, err.Error())
		}
		key := consAddr.String()

		if update.Power == 0 {
			// Remove validator if power is 0
			delete(validatorMap, key)
		} else {
			// Add or update validator
			validatorMap[key] = &ValidatorInfo{
				MinaAddress: string(minaPubKey),
				Power:       update.Power,
			}
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
			return nil, errors.Wrap(types.ErrFailedToConvertPubKeyToAddr, err.Error())
		}

		input = append(input, MinaPublicKey.X)
		if MinaPublicKey.IsOdd {
			input = append(input, big.NewInt(1))
		} else {
			input = append(input, big.NewInt(0))
		}
		power := new(big.Int).SetInt64(validator.Power)
		input = append(input, power)

		// Hash the validator address
		hashOfAddr := poseidonHash.Hash(input)

		// Append the merkle root and the hash of the validator address to the input
		input = []*big.Int{merkleRoot, hashOfAddr}

		// Hash the input to get the new merkle root
		merkleRoot = poseidonHash.Hash(input)
	}

	return merkleRoot, nil
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

func (h *VoteExtHandler) getVoteExtBody(height uint64) (types.Body, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	addr, err := h.MinaPrivateKey.PublicKey.ToAddress()
	if err != nil {
		return types.Body{}, errors.Wrap(types.ErrFailedToConvertPubKeyToAddr, "nodes own secondary pubkey")
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
	return types.Body{}, errors.Wrap(types.ErrMissingVoteExt, "")
}
