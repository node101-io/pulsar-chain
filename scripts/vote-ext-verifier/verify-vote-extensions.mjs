#!/usr/bin/env node

import {
  fetchPersistedVoteExtensions,
  fetchVoteExtBody,
  recomputeReferenceRoot,
} from './request-logic.mjs';
import {
  decodeBase64,
  decodeMinaPublicKey,
  decodeMinaSignature,
  fieldFromBigEndian,
  fieldFromCanonicalBytes,
  stateRootToField,
  verifyVotes,
} from './verifier-logic.mjs';

const DEFAULT_GRPC_ADDR = '127.0.0.1:9090';
const DEFAULT_RPC_BASE = 'http://127.0.0.1:26657';

const VOTE_EXTENSIONS_METHOD = 'pulsarchain.votepersistence.v1.Query/VoteExtensions';
const VOTE_EXT_BODY_BY_HEIGHT_METHOD = 'pulsarchain.abci.Query/VoteExtBodyByHeight';
const VALIDATOR_MINA_KEY_METHOD = 'pulsarchain.keyregistry.v1.Query/GetValidatorMinaPubKey';

const VALIDATOR_LEAF_PREFIX = 'pulsar-validator';
const VERIFICATION_MODE = 'Signature.verify([voteExtBodyHash])';

function printUsage() {
  console.log(`Usage:
  node verify-vote-extensions.mjs [--grpc ADDR] [--rpc URL] [--json]

What it does:
  1. Queries the current persisted vote extensions
  2. Fetches the matching vote-extension body
  3. Recomputes validatorSetRoot with the attached field-based o1js logic
  4. Verifies signatures with o1js Signature.verify over the voteExtBody hash field

Examples:
  node verify-vote-extensions.mjs --grpc 127.0.0.1:9090 --rpc http://127.0.0.1:26657
  node verify-vote-extensions.mjs --grpc 127.0.0.1:9090 --rpc http://127.0.0.1:26657 --json
`);
}

function parseArgs(argv) {
  const options = {
    grpcAddr: DEFAULT_GRPC_ADDR,
    rpcBase: DEFAULT_RPC_BASE,
    json: false,
    help: false,
  };

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];

    if (arg === '--help' || arg === '-h') {
      options.help = true;
      continue;
    }

    if (arg === '--json') {
      options.json = true;
      continue;
    }

    if (arg === '--grpc' || arg === '--rpc') {
      const value = argv[i + 1];
      if (!value) {
        throw new Error(`missing value for ${arg}`);
      }

      if (arg === '--grpc') options.grpcAddr = value;
      if (arg === '--rpc') options.rpcBase = value;
      i += 1;
      continue;
    }

    throw new Error(`unknown argument: ${arg}`);
  }

  return options;
}

function serializeVote(index, vote, signatureValid, validatorInfo) {
  return {
    index,
    minaPublicKeyBase64: vote.mina_public_key,
    voteExtensionBase64: vote.vote_extension,
    signatureValid,
    validatorMatched: validatorInfo !== null,
    validatorPower: validatorInfo?.power ?? null,
    consensusAddress: validatorInfo?.address ?? null,
  };
}

async function buildReport(options) {
  const votesResponse = await fetchPersistedVoteExtensions(
    options.grpcAddr,
    VOTE_EXTENSIONS_METHOD
  );
  const votes = Array.isArray(votesResponse?.voteExtensions)
    ? votesResponse.voteExtensions
    : [];

  if (votes.length === 0) {
    throw new Error(
      `no persisted vote extensions found at query height ${votesResponse?.queryBlockHeight ?? 'unknown'}`
    );
  }

  const queryBlockHeight = BigInt(votesResponse.queryBlockHeight);
  const signedStateHeight = BigInt(votesResponse.persistedVoteExtensionsBlockHeight);
  const voteExtensionHeight = signedStateHeight + 2n;

  const bodyResponse = await fetchVoteExtBody(
    options.grpcAddr,
    voteExtensionHeight.toString(),
    VOTE_EXT_BODY_BY_HEIGHT_METHOD
  );
  const body = bodyResponse?.voteExtBody;
  if (!body) {
    throw new Error('vote extension body response is empty');
  }

  const bodyCurrentBlockHeight = BigInt(body.currentBlockHeight);
  if (bodyCurrentBlockHeight !== signedStateHeight) {
    throw new Error(
      `body current_block_height mismatch: got ${bodyCurrentBlockHeight}, want ${signedStateHeight}`
    );
  }

  const nextValidatorSetHashBytes = decodeBase64(
    body.nextValidatorSetHash,
    'voteExtBody.nextValidatorSetHash'
  );
  const currentStateRootBytes = decodeBase64(
    body.currentStateRoot,
    'voteExtBody.currentStateRoot'
  );
  const actionsReducedRootBytes = decodeBase64(
    body.actionsReducedRoot,
    'voteExtBody.actionsReducedRoot'
  );
  const actionsReducedRoot = fieldFromCanonicalBytes(
    actionsReducedRootBytes,
    'voteExtBody.actionsReducedRoot'
  );

  const reference = await recomputeReferenceRoot({
    grpcAddr: options.grpcAddr,
    rpcBase: options.rpcBase,
    voteExtensionHeight: voteExtensionHeight.toString(),
    validatorMinaKeyMethod: VALIDATOR_MINA_KEY_METHOD,
  });

  const bodyForVerifier = {
    validatorSetRoot: fieldFromBigEndian(nextValidatorSetHashBytes),
    stateRoot: stateRootToField(currentStateRootBytes),
    blockHeight: bodyCurrentBlockHeight,
    actionsReducedRoot,
  };

  const verifierVotes = votes.map((vote, index) => {
    const minaPublicKeyBytes = decodeBase64(
      vote.minaPublicKey,
      `voteExtensions[${index}].minaPublicKey`
    );
    const voteExtensionBytes = decodeBase64(
      vote.voteExtension,
      `voteExtensions[${index}].voteExtension`
    );
    return {
      publicKey: decodeMinaPublicKey(minaPublicKeyBytes),
      signature: decodeMinaSignature(voteExtensionBytes),
    };
  });

  const verification = verifyVotes({
    validators: reference.validators,
    body: bodyForVerifier,
    votes: verifierVotes,
    validatorLeafPrefix: VALIDATOR_LEAF_PREFIX,
    verificationMode: VERIFICATION_MODE,
  });

  const votesByMinaKey = reference.validatorsByMinaKey;
  const voteResults = votes.map((vote, index) =>
    serializeVote(
      index + 1,
      {
        mina_public_key: vote.minaPublicKey,
        vote_extension: vote.voteExtension,
      },
      verification.signatures[index],
      votesByMinaKey.get(vote.minaPublicKey) ?? null
    )
  );

  const overallValid = verification.rootOk && verification.signatures.every(Boolean);

  return {
    ok: overallValid,
    grpcAddr: options.grpcAddr,
    rpcBase: options.rpcBase || null,
    queryBlockHeight: queryBlockHeight.toString(),
    signedStateHeight: signedStateHeight.toString(),
    voteExtensionHeight: voteExtensionHeight.toString(),
    body: {
      currentBlockHeight: bodyCurrentBlockHeight.toString(),
      currentStateRootBase64: body.currentStateRoot,
      nextValidatorSetHashBase64: body.nextValidatorSetHash,
      actionsReducedRootBase64: body.actionsReducedRoot,
      validatorSetRootField: bodyForVerifier.validatorSetRoot.toString(),
      stateRootField: bodyForVerifier.stateRoot.toString(),
      actionsReducedRootField: bodyForVerifier.actionsReducedRoot.toString(),
      voteExtBodyHashField: verification.msg.toString(),
    },
    verification: {
      rootOk: verification.rootOk,
      recomputedRoot: verification.recomputedRoot.toString(),
      validatorCount: reference.validatorCount,
      validatorKeysResolved: reference.validatorKeysResolved,
      verificationMode: verification.verificationMode,
    },
    votes: voteResults,
  };
}

function printTextReport(report) {
  console.log(`grpc_addr: ${report.grpcAddr}`);
  console.log(`rpc_base: ${report.rpcBase}`);
  console.log(`query_block_height: ${report.queryBlockHeight}`);
  console.log(`signed_state_height: ${report.signedStateHeight}`);
  console.log(`vote_extension_height: ${report.voteExtensionHeight}`);
  console.log('');
  console.log(`body.current_block_height: ${report.body.currentBlockHeight}`);
  console.log(`body.current_state_root(base64): ${report.body.currentStateRootBase64}`);
  console.log(
    `body.next_validator_set_hash(base64): ${report.body.nextValidatorSetHashBase64}`
  );
  console.log(
    `body.actions_reduced_root(base64): ${report.body.actionsReducedRootBase64}`
  );
  console.log(`body.validator_set_root(field): ${report.body.validatorSetRootField}`);
  console.log(`body.state_root(field): ${report.body.stateRootField}`);
  console.log(`body.actions_reduced_root(field): ${report.body.actionsReducedRootField}`);
  console.log(`body.vote_ext_body_hash(field): ${report.body.voteExtBodyHashField}`);
  console.log('');
  console.log(`reference_root_check: ${report.verification.rootOk ? 'match' : 'mismatch'}`);
  console.log(`reference_root(field): ${report.verification.recomputedRoot}`);
  console.log(
    `validators_resolved: ${report.verification.validatorKeysResolved}/${report.verification.validatorCount}`
  );
  console.log(`signature_mode: ${report.verification.verificationMode}`);

  console.log('');
  for (const vote of report.votes) {
    const status = vote.signatureValid ? 'ok' : 'FAIL';
    const validatorInfo = vote.validatorMatched
      ? ` power=${vote.validatorPower} consensus_address=${vote.consensusAddress}`
      : '';
    console.log(
      `vote ${vote.index}: ${status}${validatorInfo} mina_public_key=${vote.minaPublicKeyBase64}`
    );
  }

  console.log('');
  console.log(
    report.ok
      ? `verified ${report.votes.length}/${report.votes.length} vote extensions`
      : `verification failed for one or more checks`
  );
}

async function main() {
  const options = parseArgs(process.argv.slice(2));
  if (options.help) {
    printUsage();
    return;
  }

  const report = await buildReport(options);
  if (options.json) {
    console.log(JSON.stringify(report, null, 2));
    if (!report.ok) {
      process.exitCode = 1;
    }
    return;
  }

  printTextReport(report);
  if (!report.ok) {
    process.exitCode = 1;
  }
}

main().catch((error) => {
  console.error(`error: ${error.message}`);
  process.exitCode = 1;
});
