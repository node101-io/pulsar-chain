package abci

import (
	"bytes"
	"fmt"

	"github.com/bronlabs/bron-crypto/pkg/signatures/schnorrlike/mina"
	"github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
	"github.com/node101-io/mina-signer-go/privatekey"
	"github.com/node101-io/mina-signer-go/publickey"
	minasignature "github.com/node101-io/mina-signer-go/signature"
	votepersistenceTypes "github.com/node101-io/pulsar-chain/x/votepersistence/types"
)

type SecondaryKey struct {
	SecretKey *privatekey.PrivateKey
	PublicKey *publickey.PublicKey
}

func (s SecondaryKey) Validate() error {
	if s.SecretKey == nil {
		return ErrMissingSecondaryKey
	}
	if s.PublicKey == nil {
		return ErrMissingSecondaryKey
	}

	derivedPublicKey, err := s.SecretKey.ToPublicKey()
	if err != nil {
		return fmt.Errorf("%w: failed to derive public key: %v", ErrInvalidSecondaryKey, err)
	}
	if !bytes.Equal(s.PublicKey.Bytes(), derivedPublicKey.Bytes()) {
		return fmt.Errorf("%w: public key does not match private key", ErrInvalidSecondaryKey)
	}

	return nil
}

func (s SecondaryKey) SignVoteExtBody(voteExtBody votepersistenceTypes.VoteExtBody) ([]byte, error) {
	if s.SecretKey == nil {
		return nil, ErrMissingSecondaryKey
	}

	poseidonHash := poseidon.NewPoseidon()

	msgHash, err := hashVoteExtBody(poseidonHash, voteExtBody)
	if err != nil {
		return nil, err
	}

	sig, err := s.SecretKey.SignFieldElement(msgHash)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVoteExtSigningFailed, err)
	}

	return sig.Bytes(), nil
}

func verifyVoteExtSig(poseidonHash *poseidon.Poseidon, signature []byte, message votepersistenceTypes.VoteExtBody, minaKey []byte, networkID mina.NetworkID) error {
	pubKey, err := publickey.NewPublicKeyFromBytes(minaKey, networkID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVoteExtMinaPublicKey, err)
	}

	msgHash, err := hashVoteExtBody(poseidonHash, message)
	if err != nil {
		return err
	}

	sig, err := minasignature.NewSignatureFromBytes(signature)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVoteExtSignatureEncoding, err)
	}

	valid, err := pubKey.VerifyField(sig, msgHash)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVoteExtSignature, err)
	}
	if !valid {
		return ErrInvalidVoteExtSignature
	}

	return nil
}

func hashVoteExtBody(poseidonHash *poseidon.Poseidon, voteExtBody votepersistenceTypes.VoteExtBody) (*field.FieldElement, error) {
	if poseidonHash == nil {
		return nil, ErrVoteExtBodyHashFailed
	}

	if voteExtBody.CurrentBlockHeight < 0 {
		return nil, fmt.Errorf("%w: current block height must be non-negative", ErrVoteExtBodyHashFailed)
	}

	minaField := field.NewField()

	validatorSetRoot, err := minaField.FromBytes(voteExtBody.NextValidatorSetHash)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid validator set root: %v", ErrVoteExtBodyHashFailed, err)
	}

	voteExtBodyHash, err := encodeVoteExtBodyForHash(poseidonHash, voteExtBody.CurrentStateRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVoteExtBodyHashFailed, err)
	}

	actionsRoot, err := minaField.FromBytes(voteExtBody.ActionsReducedRoot)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid actions reduced root: %v", ErrVoteExtBodyHashFailed, err)
	}

	inner, err := poseidonHash.HashFieldElements(
		validatorSetRoot,
		voteExtBodyHash,
		minaField.FromUint64(uint64(voteExtBody.CurrentBlockHeight)),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVoteExtBodyHashFailed, err)
	}

	root, err := field.NewFieldElement(actionsRoot.Bytes())
	if err != nil {
		return nil, err
	}

	hash, err := poseidonHash.HashFieldElements(inner, root)
	if err != nil {
		return nil, err
	}

	return hash, nil
}

func encodeVoteExtBodyForHash(
	poseidonHash *poseidon.Poseidon,
	appHash []byte,
) (*field.FieldElement, error) {
	if len(appHash) != 32 {
		return nil, fmt.Errorf("current state root must be 32 bytes")
	}

	field := field.NewField()

	hi, err := field.FromBytesBEReduce(appHash[:16])
	if err != nil {
		return nil, err
	}

	lo, err := field.FromBytesBEReduce(appHash[16:])
	if err != nil {
		return nil, err
	}

	hashBytes, err := poseidonHash.HashFieldElements(hi, lo)
	if err != nil {
		return nil, err
	}

	return hashBytes, nil
}
