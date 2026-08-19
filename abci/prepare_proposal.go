package abci

import (
	"bytes"
	"fmt"
	"sort"

	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// PrepareProposalHandler places the consensus-internal vote-extension payload
// in the reserved first transaction slot, then fills the remaining block space
// with normal transactions. Mandatory Mina data always receives space first;
// optional verification entries may be trimmed instead of crowding out the
// chain behavior that existed before this module.
func (h *ABCIHandler) PrepareProposalHandler() sdk.PrepareProposalHandler {

	return func(ctx sdk.Context, req *cometabci.RequestPrepareProposal) (*cometabci.ResponsePrepareProposal, error) {

		shouldIncludeVoteExtensions, err := shouldRequireProposalPayloadAtHeight(ctx, req.GetHeight())
		if err != nil {
			return nil, err
		}

		if !shouldIncludeVoteExtensions {
			return &cometabci.ResponsePrepareProposal{Txs: req.Txs}, nil
		}

		votes := req.LocalLastCommit.Votes
		pl, err := h.constructPayload(ctx, req.GetHeight(), req.LocalLastCommit.Round, votes)
		if err != nil {
			return nil, err
		}
		pl, err = fitVerificationEntries(pl, req.GetHeight(), req.MaxTxBytes)
		if err != nil {
			return nil, err
		}

		bz, err := pl.Marshal()
		if err != nil {
			return nil, err
		}

		// The marker reserves the first transaction slot for the internal
		// vote-extension payload. It is not an SDK transaction and cannot be
		// submitted by a user; remaining entries are ordinary user transactions.
		extTx := append(voteExtMarkerBytes[:len(voteExtMarkerBytes):len(voteExtMarkerBytes)], bz...)
		if req.MaxTxBytes >= 0 && int64(len(extTx)) > req.MaxTxBytes {
			return nil, fmt.Errorf("%w: payload tx size %d exceeds max tx bytes %d", ErrVoteExtPayloadTooLarge, len(extTx), req.MaxTxBytes)
		}

		txs := make([][]byte, 0, len(req.Txs)+1)
		txs = append(txs, extTx)

		totalBytes := int64(len(extTx))
		for _, tx := range req.Txs {
			txSize := int64(len(tx))
			if req.MaxTxBytes >= 0 && totalBytes+txSize > req.MaxTxBytes {
				break
			}
			txs = append(txs, tx)
			totalBytes += txSize
		}

		return &cometabci.ResponsePrepareProposal{Txs: txs}, nil
	}
}

// fitVerificationEntries keeps the mandatory Mina payload intact and selects
// only optional verification entries that fit MaxTxBytes. Because every entry
// contains signatures and revelations, this bound protects proposal capacity.
// Rotation by proposal height avoids always favoring low-address validators
// when the block cannot carry every optional entry.
func fitVerificationEntries(payload Payload, proposalHeight, maxTxBytes int64) (Payload, error) {
	candidates := payload.VerificationEntries
	payload.VerificationEntries = nil
	mandatorySize := int64(payload.Size())
	totalSize := int64(len(voteExtMarkerBytes)) + mandatorySize
	if maxTxBytes >= 0 && totalSize > maxTxBytes {
		return Payload{}, fmt.Errorf("%w: mandatory payload exceeds max tx bytes", ErrVoteExtPayloadTooLarge)
	}
	if len(candidates) == 0 {
		return payload, nil
	}

	start := int(proposalHeight % int64(len(candidates)))
	selected := make([]*PayloadVerificationEntry, 0, len(candidates))
	for offset := range candidates {
		candidate := candidates[(start+offset)%len(candidates)]
		if candidate == nil {
			return Payload{}, ErrInvalidVerificationPayload
		}
		entrySize := candidate.Size()
		encodedSize := int64(1 + protobufVarintSize(uint64(entrySize)) + entrySize)
		if maxTxBytes < 0 || totalSize+encodedSize <= maxTxBytes {
			selected = append(selected, candidate)
			totalSize += encodedSize
		}
	}
	// Selection is rotated for fairness, then sorted for canonical encoding so
	// all validators can enforce uniqueness independently of proposer iteration.
	sort.Slice(selected, func(i, j int) bool {
		return bytes.Compare(selected[i].ValidatorAddress, selected[j].ValidatorAddress) < 0
	})
	payload.VerificationEntries = selected

	return payload, nil
}

func protobufVarintSize(value uint64) int {
	size := 1
	for value >= 1<<7 {
		value >>= 7
		size++
	}
	return size
}
