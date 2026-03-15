package types

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/mina-signer-go/poseidonbigint"
)

func (b *Body) GetPoseidonHashInput(ctx sdk.Context, poseidonHash *poseidon.Poseidon) poseidonbigint.HashInput {
	// Initialize the input array
	input := []*big.Int{}

	input = append(input, new(big.Int).SetBytes(b.InitialValidatorSetRoot))
	input = append(input, big.NewInt(0).SetBytes(b.InitialStateRoot))
	input = append(input, big.NewInt(b.InitialBlockHeight))
	input = append(input, new(big.Int).SetBytes(b.NewValidatorSetRoot))
	input = append(input, big.NewInt(0).SetBytes(b.NewStateRoot))
	input = append(input, big.NewInt(b.NewBlockHeight))

	// Hash the vote extension body
	hashOfBody := poseidonHash.Hash(input)

	return poseidonbigint.HashInput{
		Fields: []*big.Int{hashOfBody},
	}
}
