package keeper_test

import (
	"fmt"
	"testing"

	querytypes "github.com/cosmos/cosmos-sdk/types/query"

	"github.com/node101-io/pulsar-chain/x/verification/keeper"
	"github.com/node101-io/pulsar-chain/x/verification/types"
)

func BenchmarkEndBlock(b *testing.B) {
	for _, proofCount := range []int{0, 1, 256} {
		b.Run(fmt.Sprintf("%d_proofs", proofCount), func(b *testing.B) {
			b.ReportAllocs()
			for iteration := 0; iteration < b.N; iteration++ {
				b.StopTimer()
				fixture := initFixture(b, 3)
				for i := 0; i < proofCount; i++ {
					submitProof(b, fixture, 500, byte(i))
				}
				b.StartTimer()
				if err := fixture.keeper.EndBlock(fixture.atHeight(505)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRevealValidation(b *testing.B) {
	for _, proofCount := range []int{1, 256} {
		b.Run(fmt.Sprintf("%d_votes", proofCount), func(b *testing.B) {
			fixture := initFixture(b, 3)
			votes := make([]types.ProofVote, proofCount)
			for i := range votes {
				submitProof(b, fixture, 500, byte(i))
				votes[i] = types.ProofVote{IndexInBlock: uint32(i), Result: i%2 == 0}
			}
			left := valueLeaf(b, 1, votes)
			right := valueLeaf(b, 2, nil)
			submitCommitment(b, fixture, 0, 503, left, right)
			revelation := types.CommitmentRevelation{
				CommitmentHeight: 503, Left: left, Right: hashLeaf(b, right),
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				plan, err := fixture.keeper.ValidateCommitmentRevelation(fixture.atHeight(504), fixture.validators[0].operator, 504, revelation)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := fixture.keeper.ValidateRevelationPlans(fixture.atHeight(504), []keeper.RevelationPlan{plan}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkApplyVotes(b *testing.B) {
	b.Run("new", func(b *testing.B) {
		for range b.N {
			b.StopTimer()
			fixture := initFixture(b, 3)
			submitProof(b, fixture, 500, 1)
			left := valueLeaf(b, 1, nil)
			right := valueLeaf(b, 2, []types.ProofVote{{IndexInBlock: 0, Result: true}})
			submitCommitment(b, fixture, 0, 502, left, right)
			revelation := types.CommitmentRevelation{
				CommitmentHeight: 502, Left: hashLeaf(b, left), Right: right,
			}
			b.StartTimer()
			if err := fixture.keeper.ApplyVerificationPayload(
				fixture.atHeight(504), fixture.validators[0].operator, 504, nil,
				[]types.CommitmentRevelation{revelation},
			); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("idempotent", func(b *testing.B) {
		fixture := initFixture(b, 3)
		submitProof(b, fixture, 500, 1)
		left := valueLeaf(b, 1, nil)
		right := valueLeaf(b, 2, []types.ProofVote{{IndexInBlock: 0, Result: true}})
		submitCommitment(b, fixture, 0, 502, left, right)
		revelation := types.CommitmentRevelation{
			CommitmentHeight: 502, Left: hashLeaf(b, left), Right: right,
		}
		requireApplyPayload(b, fixture, revelation)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			requireApplyPayload(b, fixture, revelation)
		}
	})

	b.Run("equivocation", func(b *testing.B) {
		for range b.N {
			b.StopTimer()
			fixture := initFixture(b, 3)
			submitProof(b, fixture, 500, 1)
			empty := valueLeaf(b, 1, nil)
			trueLeaf := valueLeaf(b, 2, []types.ProofVote{{IndexInBlock: 0, Result: true}})
			falseLeaf := valueLeaf(b, 3, []types.ProofVote{{IndexInBlock: 0, Result: false}})
			submitCommitment(b, fixture, 0, 502, empty, trueLeaf)
			submitCommitment(b, fixture, 0, 503, falseLeaf, empty)
			requireApplyPayload(b, fixture, types.CommitmentRevelation{
				CommitmentHeight: 502, Left: hashLeaf(b, empty), Right: trueLeaf,
			})
			conflict := types.CommitmentRevelation{
				CommitmentHeight: 503, Left: falseLeaf, Right: hashLeaf(b, empty),
			}
			b.StartTimer()
			requireApplyPayload(b, fixture, conflict)
		}
	})

	b.Run("max", func(b *testing.B) {
		for range b.N {
			b.StopTimer()
			fixture := initFixture(b, 3)
			votes := make([]types.ProofVote, types.MaxVoteIndexExclusive)
			for i := range votes {
				submitProof(b, fixture, 500, byte(i))
				votes[i] = types.ProofVote{IndexInBlock: uint32(i), Result: i%2 == 0}
			}
			left := valueLeaf(b, 1, votes)
			right := valueLeaf(b, 2, nil)
			submitCommitment(b, fixture, 0, 503, left, right)
			revelation := types.CommitmentRevelation{
				CommitmentHeight: 503, Left: left, Right: hashLeaf(b, right),
			}
			b.StartTimer()
			requireApplyPayload(b, fixture, revelation)
		}
	})
}

func BenchmarkQueries(b *testing.B) {
	fixture := initFixture(b, 3)
	for i := 0; i < int(types.MaxVoteIndexExclusive); i++ {
		submitProof(b, fixture, 500, byte(i))
	}
	server := keeper.NewQueryServerImpl(fixture.keeper)

	b.Run("single", func(b *testing.B) {
		request := &types.QueryProofRequest{SubmissionHeight: 500, IndexInBlock: 128}
		b.ReportAllocs()
		for range b.N {
			if _, err := server.Proof(fixture.atHeight(500), request); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("paginated", func(b *testing.B) {
		request := &types.QueryProofsByHeightRequest{
			SubmissionHeight: 500,
			Pagination:       &querytypes.PageRequest{Limit: 50},
		}
		b.ReportAllocs()
		for range b.N {
			if _, err := server.ProofsByHeight(fixture.atHeight(500), request); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func requireApplyPayload(b *testing.B, fixture *fixture, revelation types.CommitmentRevelation) {
	b.Helper()
	if err := fixture.keeper.ApplyVerificationPayload(
		fixture.atHeight(504), fixture.validators[0].operator, 504, nil,
		[]types.CommitmentRevelation{revelation},
	); err != nil {
		b.Fatal(err)
	}
}
