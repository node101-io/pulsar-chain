import { Bool, Field, Poseidon, PublicKey, Signature } from 'o1js';

export function decodeBase64(value, label) {
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error(`missing ${label}`);
  }

  return Buffer.from(value, 'base64');
}

export function decodeHex(value, label) {
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error(`missing ${label}`);
  }

  if (value.length % 2 !== 0 || !/^[0-9a-fA-F]+$/.test(value)) {
    throw new Error(`invalid hex for ${label}`);
  }

  return Buffer.from(value, 'hex');
}

function bigIntFromBytesLE(bytes) {
  let value = 0n;
  for (let i = bytes.length - 1; i >= 0; i -= 1) {
    value = (value << 8n) | BigInt(bytes[i]);
  }
  return value;
}

function bigIntFromBytesBE(bytes) {
  let value = 0n;
  for (const byte of bytes) {
    value = (value << 8n) | BigInt(byte);
  }
  return value;
}

function fieldFromLittleEndian(bytes) {
  return Field(bigIntFromBytesLE(bytes));
}

export function fieldFromBigEndian(bytes) {
  return Field(bigIntFromBytesBE(bytes));
}

export function decodeMinaPublicKey(rawBytes) {
  if (rawBytes.length !== 32) {
    throw new Error(`invalid Mina public key length: got ${rawBytes.length}, want 32`);
  }

  const compressed = Uint8Array.from(rawBytes);
  const isOdd = (compressed[31] & 0x80) !== 0;
  compressed[31] &= 0x7f;

  return PublicKey.from({
    x: fieldFromLittleEndian(compressed),
    isOdd: Bool(isOdd),
  });
}

export function decodeMinaSignature(rawBytes) {
  if (rawBytes.length !== 64) {
    throw new Error(`invalid Mina signature length: got ${rawBytes.length}, want 64`);
  }

  return Signature.fromValue({
    r: bigIntFromBytesLE(rawBytes.subarray(0, 32)),
    s: bigIntFromBytesLE(rawBytes.subarray(32, 64)),
  });
}

function validatorLeaf({ publicKey, power }, validatorLeafPrefix) {
  const [x, isOdd] = publicKey.toFields();
  return Poseidon.hashWithPrefix(validatorLeafPrefix, [x, isOdd, Field(power)]);
}

export function validatorSetRoot(validators, validatorLeafPrefix) {
  const sorted = [...validators].sort((a, b) => {
    if (a.power < b.power) return -1;
    if (a.power > b.power) return 1;

    return Buffer.compare(a.consensusAddressBytes, b.consensusAddressBytes);
  });

  let acc = Poseidon.hash([Field(0)]);
  for (const validator of sorted) {
    acc = Poseidon.hash([acc, validatorLeaf(validator, validatorLeafPrefix)]);
  }
  return acc;
}

export function stateRootToField(appHash32) {
  if (appHash32.length !== 32) {
    throw new Error(`invalid AppHash length: got ${appHash32.length}, want 32`);
  }

  return Poseidon.hash([
    fieldFromBigEndian(appHash32.subarray(0, 16)),
    fieldFromBigEndian(appHash32.subarray(16, 32)),
  ]);
}

export function voteExtBodyHash({
  validatorSetRoot,
  stateRoot,
  blockHeight,
  actionsReducedRoot,
}) {
  const inner = Poseidon.hash([validatorSetRoot, stateRoot, Field(blockHeight)]);
  return Poseidon.hash([inner, actionsReducedRoot]);
}

function verifySignatures(msg, votes) {
  return votes.map(({ publicKey, signature }) => signature.verify(publicKey, [msg]).toBoolean());
}

export function verifyVotes({
  validators,
  body,
  votes,
  validatorLeafPrefix,
  verificationMode,
}) {
  const recomputedRoot = validatorSetRoot(validators, validatorLeafPrefix);
  const rootOk = recomputedRoot.equals(body.validatorSetRoot).toBoolean();
  const msg = voteExtBodyHash({ ...body, validatorSetRoot: recomputedRoot });

  return {
    rootOk,
    signatures: verifySignatures(msg, votes),
    recomputedRoot,
    msg,
    verificationMode,
  };
}
