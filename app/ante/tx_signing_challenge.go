package ante

import (
	"bytes"
	"encoding/binary"

	errorsmod "cosmossdk.io/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	minafield "github.com/node101-io/mina-signer-go/field"
	"github.com/node101-io/mina-signer-go/poseidon"
)

// The field element a Mina wallet signs to authorize a Pulsar transaction.
//
// A wallet is the whole reason this derivation exists: browser wallets expose
// field signing (Auro's signFields) but never raw byte signing over arbitrary
// payloads — that would take the private key out of the wallet's hands. So the
// transaction's canonical sign bytes are reduced to one field element here,
// the wallet signs that field, and verifySingleSignature recomputes the same
// field from the same bytes. Clients MUST reproduce this derivation
// byte-for-byte; it is intentionally shaped like the key-registry challenges
// in x/keyregistry/types/signing.go so both sides of the bridge maintain one
// idiom:
//
//	challenge = PoseidonHashWithPrefix(prefix, version || len(signBytes) || signBytes)
//
// The sign bytes already bind chain ID, account number, sequence, fee and
// every message, so the challenge inherits replay protection from the same
// place Cosmos signatures get it.
//
// The prefix domain-separates this signature from everything else a Mina
// wallet is ever asked to sign on Pulsar — see the registration/update
// prefixes in x/keyregistry/types/signing.go. Every such prefix must be
// globally unique: two contexts sharing one prefix would let a signature
// gathered in one be replayed in the other. Never reuse or retire a prefix;
// a derivation change gets a NEW versioned prefix instead.
const txSigningChallengePrefix = "pulsar-tx-auth-v1"

const txSigningChallengeVersion byte = 1

// BuildTxSigningChallenge derives the field element a Mina wallet signs for
// the given canonical sign bytes. Exported for the ante tests and as the
// reference for client-side mirrors (pulsar-chain-client pins this via test
// vectors, the way keySigningChallenge is pinned).
func BuildTxSigningChallenge(signBytes []byte) (*minafield.FieldElement, error) {
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
