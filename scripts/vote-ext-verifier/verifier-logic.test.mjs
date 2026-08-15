import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

import { Field, Poseidon } from 'o1js';

import {
  fieldFromCanonicalBytes,
  stateRootToField,
  voteExtBodyHash,
} from './verifier-logic.mjs';

function loadJSON(filename) {
  return JSON.parse(readFileSync(new URL(filename, import.meta.url), 'utf8'));
}

function bigIntToBytesBE(value) {
  return Buffer.from(value.toString(16).padStart(64, '0'), 'hex');
}

test('fieldFromCanonicalBytes decodes the shared action-root vectors', async (t) => {
  const vectors = loadJSON('./actions-root-vectors.json');

  for (const [name, vector] of Object.entries({ emptyRoot: vectors.emptyRoot })) {
    await t.test(name, () => {
      const bytes = Buffer.from(vector.base64, 'base64');
      const field = fieldFromCanonicalBytes(bytes, name);

      assert.equal(field.toString(), vector.decimal);
    });
  }
});

test('o1js reconstructs the shared action hashes and roots', async (t) => {
  const vectors = loadJSON('./actions-root-vectors.json');
  const actionTypes = { ACTION_TYPE_DEPOSIT: 1, ACTION_TYPE_WITHDRAW: 2 };

  for (const vector of vectors.singleActionRoots) {
    await t.test(vector.name, () => {
      const action = vector.action;
      const xCoordinate = fieldFromCanonicalBytes(
        Buffer.from(action.xCoordinate, 'base64'),
        `${vector.name} x-coordinate`,
      );
      const actionHash = Poseidon.hashWithPrefix('pulsar_bridge_action_v1', [
        Field(action.isValidAction ? 1 : 0),
        xCoordinate,
        Field(action.isOdd ? 1 : 0),
        Field(actionTypes[action.actionType]),
        Field(action.amount),
      ]);

      assert.equal(actionHash.toString(), vector.actionHash.decimal);

      const root = Poseidon.hashWithPrefix('pulsar_bridge_actions_root_v1', [
        Field(0),
        actionHash,
      ]);
      assert.equal(root.toString(), vector.root.decimal);
    });
  }
});

test('fieldFromCanonicalBytes rejects invalid byte lengths', () => {
  assert.throws(
    () => fieldFromCanonicalBytes(Buffer.alloc(31), 'actions root'),
    /invalid actions root length: got 31, want 32/,
  );
  assert.throws(
    () => fieldFromCanonicalBytes(Buffer.alloc(33), 'actions root'),
    /invalid actions root length: got 33, want 32/,
  );
});

test('fieldFromCanonicalBytes enforces the canonical field modulus', () => {
  const largestCanonical = Field.ORDER - 1n;

  assert.equal(
    fieldFromCanonicalBytes(
      bigIntToBytesBE(largestCanonical),
      'actions root',
    ).toString(),
    largestCanonical.toString(),
  );
  assert.throws(
    () => fieldFromCanonicalBytes(bigIntToBytesBE(Field.ORDER), 'actions root'),
    /value is not a canonical Mina field element/,
  );
  assert.throws(
    () => fieldFromCanonicalBytes(bigIntToBytesBE(Field.ORDER + 1n), 'actions root'),
    /value is not a canonical Mina field element/,
  );
});

test('voteExtBodyHash matches the shared Go verifier vector', () => {
  const vector = loadJSON('./vote-ext-body-vector.json');
  const appHash = Buffer.from(vector.appHashBase64, 'base64');
  const actionsReducedRoot = fieldFromCanonicalBytes(
    Buffer.from(vector.actionsReducedRootBase64, 'base64'),
    'actions reduced root',
  );
  const stateRoot = stateRootToField(appHash);

  assert.equal(stateRoot.toString(), vector.stateRootDecimal);

  const bodyHash = voteExtBodyHash({
    validatorSetRoot: Field(vector.validatorSetRootDecimal),
    stateRoot,
    blockHeight: vector.blockHeight,
    actionsReducedRoot,
  });

  assert.equal(bodyHash.toString(), vector.bodyHashDecimal);
});
