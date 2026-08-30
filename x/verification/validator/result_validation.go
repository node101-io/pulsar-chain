package validator

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/node101-io/pulsar-chain/x/verification/sidecar"
	verificationtypes "github.com/node101-io/pulsar-chain/x/verification/types"
)

type requestedProof struct {
	height uint64
	index  uint32
}

func buildResultRequest(
	leftHeight, rightHeight uint64,
	leftProofs, rightProofs []verificationtypes.ProofEntry,
	excludedLeft map[uint32]struct{},
) ([][]byte, map[string]requestedProof, error) {
	capacity := len(leftProofs) + len(rightProofs)
	if capacity > verificationtypes.MaxVoteIndexExclusive*2 {
		return nil, nil, verificationtypes.ErrTooManyVotes
	}
	request := make([][]byte, 0, capacity)
	index := make(map[string]requestedProof, capacity)
	// Index by verification ID so the unordered sidecar response can be mapped
	// back to the canonical proof height and in-block index. The sidecar is not
	// trusted to know ProofKey or preserve request order; those are chain
	// responsibilities.
	appendProofs := func(expectedHeight uint64, proofs []verificationtypes.ProofEntry, excluded map[uint32]struct{}) error {
		for _, proof := range proofs {
			if proof.Key.SubmissionHeight != expectedHeight || proof.Key.IndexInBlock >= verificationtypes.MaxVoteIndexExclusive ||
				verificationtypes.ValidateProofRecord(proof.Record) != nil {
				return verificationtypes.ErrProofStateCorrupted
			}
			if _, skip := excluded[proof.Key.IndexInBlock]; skip {
				continue
			}
			key := string(proof.Record.VerificationId)
			if _, duplicate := index[key]; duplicate {
				return verificationtypes.ErrProofStateCorrupted
			}
			index[key] = requestedProof{height: expectedHeight, index: proof.Key.IndexInBlock}
			request = append(request, bytes.Clone(proof.Record.VerificationId))
		}
		return nil
	}
	if err := appendProofs(leftHeight, leftProofs, excludedLeft); err != nil {
		return nil, nil, err
	}
	if err := appendProofs(rightHeight, rightProofs, nil); err != nil {
		return nil, nil, err
	}
	return request, index, nil
}

func validateResults(
	requested map[string]requestedProof,
	results []sidecar.VerificationResult,
	leftHeight uint64,
) ([]verificationtypes.ProofVote, []verificationtypes.ProofVote, error) {
	if len(results) > len(requested) {
		return nil, nil, fmt.Errorf("sidecar returned more results than requested")
	}
	// Treat the response as one atomic trust-boundary object. Any malformed,
	// duplicate, unknown, or unrequested item rejects the entire response. Using
	// a partial prefix after an error could make validators commit different vote
	// sets depending on response order.
	seen := make(map[string]struct{}, len(results))
	leftVotes := make([]verificationtypes.ProofVote, 0, len(results))
	rightVotes := make([]verificationtypes.ProofVote, 0, len(results))
	for _, result := range results {
		if len(result.VerificationID) != verificationtypes.VerificationIDSize {
			return nil, nil, verificationtypes.ErrInvalidVerificationID
		}
		key := string(result.VerificationID)
		proof, ok := requested[key]
		if !ok {
			return nil, nil, fmt.Errorf("sidecar returned an unrequested verification ID")
		}
		if _, duplicate := seen[key]; duplicate {
			return nil, nil, fmt.Errorf("sidecar returned a duplicate verification ID")
		}
		seen[key] = struct{}{}
		vote := verificationtypes.ProofVote{IndexInBlock: proof.index}
		switch result.Result {
		case sidecar.ResultValid:
			vote.Result = true
		case sidecar.ResultInvalid:
			vote.Result = false
		default:
			return nil, nil, fmt.Errorf("sidecar returned an unspecified verification result")
		}
		if proof.height == leftHeight {
			leftVotes = append(leftVotes, vote)
		} else {
			rightVotes = append(rightVotes, vote)
		}
	}
	// Response order is not part of the RPC contract. Restore canonical order
	// before hashing the vote lists so identical terminal subsets always create
	// identical encoded vote lists before random salting.
	sort.Slice(leftVotes, func(i, j int) bool { return leftVotes[i].IndexInBlock < leftVotes[j].IndexInBlock })
	sort.Slice(rightVotes, func(i, j int) bool { return rightVotes[i].IndexInBlock < rightVotes[j].IndexInBlock })
	if err := verificationtypes.ValidateCanonicalVotes(leftVotes); err != nil {
		return nil, nil, err
	}
	if err := verificationtypes.ValidateCanonicalVotes(rightVotes); err != nil {
		return nil, nil, err
	}
	return leftVotes, rightVotes, nil
}
