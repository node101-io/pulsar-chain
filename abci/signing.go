package abci

import (
	"fmt"
	"math/big"

	"github.com/node101-io/mina-signer-go/constants"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/keys"
	"github.com/node101-io/mina-signer-go/poseidon"
	minasignature "github.com/node101-io/mina-signer-go/signature"
	votepersistenceTypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

type SecondaryKey struct {
	SecretKey *keys.PrivateKey
	PublicKey *keys.PublicKey
}

func (s SecondaryKey) Validate() error {
	if s.SecretKey == nil || s.SecretKey.Value == nil {
		return ErrMissingSecondaryKey
	}
	if s.SecretKey.Value.Sign() == 0 {
		return fmt.Errorf("%w: private key value must be non-zero", ErrInvalidSecondaryKey)
	}
	if s.PublicKey == nil || s.PublicKey.X == nil {
		return ErrMissingSecondaryKey
	}

	derivedPublicKey := s.SecretKey.ToPublicKey()
	if !s.PublicKey.Equal(derivedPublicKey) {
		return fmt.Errorf("%w: public key does not match private key", ErrInvalidSecondaryKey)
	}

	return nil
}

func (s SecondaryKey) SignVoteExtBody(voteExtBody votepersistenceTypes.VoteExtBody) ([]byte, error) {
	if s.SecretKey == nil || s.SecretKey.Value == nil {
		return nil, ErrMissingSecondaryKey
	}
	if s.SecretKey.Value.Sign() == 0 {
		return nil, fmt.Errorf("%w: private key value must be non-zero", ErrInvalidSecondaryKey)
	}

	poseidonHash := poseidon.CreatePoseidon(*field.Fp, constants.PoseidonParamsKimchiFp)
	msgHash, err := hashVoteExtBody(poseidonHash, voteExtBody)
	if err != nil {
		return nil, err
	}

	sig, err := s.SecretKey.SignFieldElement(msgHash, NetworkID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVoteExtSigningFailed, err)
	}

	bz, err := sig.MarshalBytes()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVoteExtSignatureMarshalFailed, err)
	}

	return bz, nil
}

func verifyVoteExtSig(poseidon *poseidon.Poseidon, signature []byte, message votepersistenceTypes.VoteExtBody, minaKey []byte, reducedRoot string) error {
	if message.ActionsReducedRoot != reducedRoot {
		return ErrInvalidVoteExtReducedRoot
	}

	var pubKey keys.PublicKey
	if err := pubKey.Unmarshal(minaKey); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVoteExtMinaPublicKey, err)
	}

	msgHash, err := hashVoteExtBody(poseidon, message)
	if err != nil {
		return err
	}

	var sig minasignature.Signature
	if err := sig.UnmarshalBytes(signature); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVoteExtSignatureEncoding, err)
	}

	if !pubKey.VerifyFieldElement(&sig, msgHash, NetworkID) {
		return ErrInvalidVoteExtSignature
	}

	return nil
}

func hashVoteExtBody(poseidonHash *poseidon.Poseidon, voteExtBody votepersistenceTypes.VoteExtBody) (*big.Int, error) {
	if poseidonHash == nil {
		return nil, ErrVoteExtBodyHashFailed
	}

	innerHash := poseidonHash.Hash([]*big.Int{
		new(big.Int).SetBytes(voteExtBody.NextValidatorSetHash),
		new(big.Int).SetBytes(voteExtBody.CurrentStateRoot),
		big.NewInt(voteExtBody.CurrentBlockHeight),
	})
	if innerHash == nil {
		return nil, ErrVoteExtBodyHashFailed
	}

	msgHash := poseidonHash.Hash([]*big.Int{
		innerHash,
		new(big.Int).SetBytes([]byte(voteExtBody.ActionsReducedRoot)),
	})
	if msgHash == nil {
		return nil, ErrVoteExtBodyHashFailed
	}

	return msgHash, nil
}
