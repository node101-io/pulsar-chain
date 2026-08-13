package types

import (
	"cosmossdk.io/errors"
	minafield "github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
)

// Poseidon domain prefixes for the registration challenge. The actor is part
// of the HASHED MESSAGE rather than the signature's network id, and that
// placement is the whole point: a Mina wallet derives its network id from the
// network it is connected to and exposes no way for a dApp to override it, so
// a domain encoded there is a domain no wallet can produce. Encoded here the
// same separation holds — a user's signature cannot be replayed as a
// validator registration — while any wallet that can sign field elements can
// produce it.
const (
	RegistrationPrefixUser      = "pulsar-keyreg-user"
	RegistrationPrefixValidator = "pulsar-keyreg-val"
)

// RegistrationChallenge returns the single Mina field element a registering
// key must sign to prove it belongs with the given Cosmos public key.
//
// SINGLE SOURCE OF TRUTH for the registration signing convention. Clients
// reproduce it in o1js with the same two calls this uses:
//
//	Poseidon.hashWithPrefix(prefix, Encoding.bytesToFields(cosmosPublicKey))
//
// then sign it with `signFields([challenge])` — `window.mina.signFields` in a
// wallet. HashWithPrefix packs the bytes exactly as o1js's bytesToFields does
// (31-byte little-endian chunks, stop byte on the last), so no splitting or
// padding is needed here and the packing stays injective: keys of different
// lengths cannot collide.
//
// A field element signed with signFields, rather than bytes signed with
// signMessage, because that is the one signing scheme the ecosystem shares:
// o1js's signMessage uses the legacy ROInput packing, which mina-signer-go's
// byte and string signing do NOT reproduce, so a signature made one way never
// verifies the other whatever the domain. A signature from a real wallet is
// pinned against this function in wallet_signature_test.go.
func RegistrationChallenge(actorType ActorType, cosmosPublicKey []byte) (*minafield.FieldElement, error) {
	var prefix string
	switch actorType {
	case ActorType_USER:
		prefix = RegistrationPrefixUser
	case ActorType_VALIDATOR:
		prefix = RegistrationPrefixValidator
	default:
		return nil, errors.Wrapf(
			ErrInvalidActorType,
			"cannot build a registration challenge for actor type %s",
			actorType,
		)
	}

	if len(cosmosPublicKey) == 0 {
		return nil, errors.Wrap(ErrInvalidPublicKey, "cosmos public key is empty")
	}

	hashed, err := poseidon.NewPoseidon().HashWithPrefix(prefix, cosmosPublicKey)
	if err != nil {
		return nil, errors.Wrapf(ErrInvalidSignature, "failed to hash registration challenge: %v", err)
	}

	challenge, err := minafield.NewField().FromBytes(hashed)
	if err != nil {
		return nil, errors.Wrapf(ErrInvalidSignature, "registration challenge is not a field element: %v", err)
	}

	return challenge, nil
}
