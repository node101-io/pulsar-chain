package types

import (
	"bytes"
	"encoding/binary"

	errorsmod "cosmossdk.io/errors"
	minafield "github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
)

const signingFormatVersion byte = 1

const (
	userRegistrationPrefix      = "pulsar-kr-user-reg-v1"
	userUpdatePrefix            = "pulsar-kr-user-upd-v1"
	validatorRegistrationPrefix = "pulsar-kr-val-reg-v1"
	validatorUpdatePrefix       = "pulsar-kr-val-upd-v1"
)

type KeySigningChallengeInput struct {
	ChainID              string
	Operation            KeySigningOperation
	ActorType            ActorType
	CosmosPublicKey      []byte
	CurrentMinaPublicKey []byte
	NewMinaPublicKey     []byte
	NewKeyVersion        uint64
}

func BuildKeySigningChallenge(input KeySigningChallengeInput) (*minafield.FieldElement, error) {
	prefix, err := validateKeySigningChallengeInput(input)
	if err != nil {
		return nil, err
	}

	payload := bytes.NewBuffer(make([]byte, 0, 1+len(input.ChainID)+len(input.CosmosPublicKey)+len(input.CurrentMinaPublicKey)+len(input.NewMinaPublicKey)+24))
	payload.WriteByte(signingFormatVersion)
	writeLengthPrefixed(payload, []byte(input.ChainID))
	writeLengthPrefixed(payload, input.CosmosPublicKey)
	writeLengthPrefixed(payload, input.CurrentMinaPublicKey)
	writeLengthPrefixed(payload, input.NewMinaPublicKey)
	if err := binary.Write(payload, binary.BigEndian, input.NewKeyVersion); err != nil {
		return nil, errorsmod.Wrapf(ErrInvalidSignature, "encode key signing challenge: %v", err)
	}

	hash, err := poseidon.NewPoseidon().HashWithPrefix(prefix, payload.Bytes())
	if err != nil {
		return nil, errorsmod.Wrapf(ErrInvalidSignature, "hash key signing challenge: %v", err)
	}

	challenge, err := minafield.NewFieldElement(hash)
	if err != nil {
		return nil, errorsmod.Wrapf(ErrInvalidSignature, "decode key signing challenge: %v", err)
	}

	return challenge, nil
}

func validateKeySigningChallengeInput(input KeySigningChallengeInput) (string, error) {
	if input.ChainID == "" {
		return "", errorsmod.Wrap(ErrInvalidSignature, "chain ID is empty")
	}

	switch input.ActorType {
	case ActorType_USER:
		if err := ValidateUserCosmosPublicKey(input.CosmosPublicKey); err != nil {
			return "", err
		}
	case ActorType_VALIDATOR:
		if err := ValidateValidatorCosmosPublicKey(input.CosmosPublicKey); err != nil {
			return "", err
		}
	default:
		return "", ErrInvalidActorType
	}

	if err := ValidateMinaPublicKey(input.NewMinaPublicKey); err != nil {
		return "", errorsmod.Wrap(err, "new mina public key")
	}

	switch input.Operation {
	case KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER:
		if len(input.CurrentMinaPublicKey) != 0 || input.NewKeyVersion != 0 {
			return "", errorsmod.Wrap(ErrInvalidKeyVersion, "registration requires an empty current key and version zero")
		}
	case KeySigningOperation_KEY_SIGNING_OPERATION_UPDATE:
		if err := ValidateMinaPublicKey(input.CurrentMinaPublicKey); err != nil {
			return "", errorsmod.Wrap(err, "current mina public key")
		}
		if input.NewKeyVersion == 0 {
			return "", errorsmod.Wrap(ErrInvalidKeyVersion, "update version must be positive")
		}
		if bytes.Equal(input.CurrentMinaPublicKey, input.NewMinaPublicKey) {
			return "", ErrUnchangedMinaPublicKey
		}
	default:
		return "", ErrInvalidSigningOperation
	}

	switch {
	case input.ActorType == ActorType_USER && input.Operation == KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER:
		return userRegistrationPrefix, nil
	case input.ActorType == ActorType_USER && input.Operation == KeySigningOperation_KEY_SIGNING_OPERATION_UPDATE:
		return userUpdatePrefix, nil
	case input.ActorType == ActorType_VALIDATOR && input.Operation == KeySigningOperation_KEY_SIGNING_OPERATION_REGISTER:
		return validatorRegistrationPrefix, nil
	case input.ActorType == ActorType_VALIDATOR && input.Operation == KeySigningOperation_KEY_SIGNING_OPERATION_UPDATE:
		return validatorUpdatePrefix, nil
	default:
		return "", errorsmod.Wrapf(ErrInvalidSigningOperation, "actor=%s operation=%s", input.ActorType, input.Operation)
	}
}

func writeLengthPrefixed(dst *bytes.Buffer, value []byte) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	dst.Write(length[:])
	dst.Write(value)
}
