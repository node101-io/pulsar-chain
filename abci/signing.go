package abci

import (
	"bytes"
	"encoding/binary"
	"fmt"

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

	sig, err := s.SecretKey.SignBytes(msgHash)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVoteExtSigningFailed, err)
	}

	return sig.Bytes(), nil
}

func verifyVoteExtSig(poseidonHash *poseidon.Poseidon, signature []byte, message votepersistenceTypes.VoteExtBody, minaKey []byte, reducedRoot string) error {
	if message.ActionsReducedRoot != reducedRoot {
		return ErrInvalidVoteExtReducedRoot
	}

	pubKey, err := publickey.NewPublicKeyFromBytes(minaKey, NetworkID)
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

	valid, err := pubKey.VerifyBytes(sig, msgHash)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVoteExtSignature, err)
	}
	if !valid {
		return ErrInvalidVoteExtSignature
	}

	return nil
}

func hashVoteExtBody(poseidonHash *poseidon.Poseidon, voteExtBody votepersistenceTypes.VoteExtBody) ([]byte, error) {
	if poseidonHash == nil {
		return nil, ErrVoteExtBodyHashFailed
	}

	if voteExtBody.CurrentBlockHeight < 0 {
		return nil, fmt.Errorf("%w: current block height must be non-negative", ErrVoteExtBodyHashFailed)
	}

	hash, err := poseidonHash.HashWithPrefix(VoteExtBodyHashPrefix, encodeVoteExtBodyForHash(voteExtBody))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVoteExtBodyHashFailed, err)
	}

	return hash, nil
}

func encodeVoteExtBodyForHash(voteExtBody votepersistenceTypes.VoteExtBody) []byte {
	var bz []byte
	bz = appendLengthPrefixedBytes(bz, voteExtBody.NextValidatorSetHash)
	bz = appendLengthPrefixedBytes(bz, voteExtBody.CurrentStateRoot)
	bz = binary.BigEndian.AppendUint64(bz, uint64(voteExtBody.CurrentBlockHeight))
	bz = appendLengthPrefixedBytes(bz, []byte(voteExtBody.ActionsReducedRoot))

	return bz
}

func encodeValidatorSetEntryForHash(minaPublicKey []byte, consensusPower int64) ([]byte, error) {
	if consensusPower < 0 {
		return nil, fmt.Errorf("%w: consensus power must be non-negative", ErrValidatorSetRootHashFailed)
	}

	var bz []byte
	bz = appendLengthPrefixedBytes(bz, minaPublicKey)
	bz = binary.BigEndian.AppendUint64(bz, uint64(consensusPower))

	return bz, nil
}

func appendLengthPrefixedBytes(dst []byte, value []byte) []byte {
	dst = binary.BigEndian.AppendUint32(dst, uint32(len(value)))
	return append(dst, value...)
}
