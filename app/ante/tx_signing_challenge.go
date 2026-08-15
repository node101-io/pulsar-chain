package ante

import (
	"bytes"
	"encoding/binary"

	errorsmod "cosmossdk.io/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	minafield "github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
)

// This transaction flow uses Auro field signing, so Pulsar clients derive the
// field below from canonical SIGN_MODE_DIRECT sign bytes:
//
//	challenge = PoseidonHashWithPrefix(prefix, version || uint32_be(len(signBytes)) || signBytes)
//
// The prefix separates transaction authorization from key registration and
// other wallet-signing contexts. Changing the prefix, version, or framing is a
// protocol change and requires new client implementations and test vectors.
const txSigningChallengePrefix = "pulsar-tx-auth-v1"

const txSigningChallengeVersion byte = 1

// TODO: Use mina-signer-go's wallet interoperability API once it provides
// Auro-compatible field and message signing with cross-language vectors.

// buildTxSigningChallenge derives the domain-separated commitment a Mina
// wallet signs for the given canonical sign bytes.
func buildTxSigningChallenge(signBytes []byte) (*minafield.FieldElement, error) {
	// Empty sign bytes cannot come out of GetSignBytesAdapter for a real tx;
	// seeing them means the caller is broken, and hashing them anyway would
	// mint a well-formed challenge for a payload that authorizes nothing.
	if len(signBytes) == 0 {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "empty sign bytes")
	}

	payload := bytes.NewBuffer(make([]byte, 0, 5+len(signBytes)))
	payload.WriteByte(txSigningChallengeVersion)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(signBytes)))
	payload.Write(length[:])
	payload.Write(signBytes)

	hash, err := poseidon.NewPoseidon().HashWithPrefix(txSigningChallengePrefix, payload.Bytes())
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "hash tx signing challenge: %v", err)
	}

	challenge, err := minafield.NewFieldElement(hash)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "decode tx signing challenge: %v", err)
	}

	return challenge, nil
}
