import { execFile } from 'node:child_process';
import { promisify } from 'node:util';

import { decodeBase64, decodeHex, decodeMinaPublicKey } from './verifier-logic.mjs';

const execFileAsync = promisify(execFile);

class HttpError extends Error {
  constructor(url, status, statusText, body) {
    const details =
      body && typeof body === 'object' && typeof body.message === 'string'
        ? `: ${body.message}`
        : '';
    super(`GET ${url} failed with ${status} ${statusText}${details}`);
    this.name = 'HttpError';
    this.url = url;
    this.status = status;
    this.statusText = statusText;
    this.body = body;
  }
}

function normalizeBaseUrl(baseUrl) {
  return baseUrl.endsWith('/') ? baseUrl : `${baseUrl}/`;
}

async function fetchJson(url) {
  let response;
  try {
    response = await fetch(url, {
      headers: {
        accept: 'application/json',
      },
    });
  } catch (error) {
    const cause = error?.cause?.message || error?.message || 'unknown network error';
    throw new Error(`request failed for ${url}: ${cause}`);
  }
  const text = await response.text();

  let body = null;
  if (text.length > 0) {
    try {
      body = JSON.parse(text);
    } catch (error) {
      throw new Error(`invalid JSON from ${url}: ${text.slice(0, 240)}`);
    }
  }

  if (!response.ok) {
    throw new HttpError(url, response.status, response.statusText, body);
  }

  return body;
}

async function grpcurlJson(grpcAddr, method, requestBody) {
  let stdout;

  try {
    ({ stdout } = await execFileAsync(
      'grpcurl',
      ['-plaintext', '-d', JSON.stringify(requestBody), grpcAddr, method],
      { maxBuffer: 10 * 1024 * 1024 }
    ));
  } catch (error) {
    const stderr = error?.stderr?.trim();
    const stdoutText = error?.stdout?.trim();
    const detail = stderr || stdoutText || error?.message || 'unknown grpcurl error';
    throw new Error(`grpcurl failed for ${method} on ${grpcAddr}: ${detail}`);
  }

  try {
    return JSON.parse(stdout);
  } catch (error) {
    throw new Error(
      `invalid JSON from grpcurl for ${method} on ${grpcAddr}: ${stdout.slice(0, 240)}`
    );
  }
}

export async function fetchPersistedVoteExtensions(grpcAddr, voteExtensionsMethod) {
  return grpcurlJson(grpcAddr, voteExtensionsMethod, {});
}

export async function fetchVoteExtBody(
  grpcAddr,
  voteExtensionHeight,
  voteExtBodyByHeightMethod
) {
  return grpcurlJson(grpcAddr, voteExtBodyByHeightMethod, {
    vote_extension_height: String(voteExtensionHeight),
  });
}

async function fetchAllValidators(rpcBase, height) {
  const validators = [];
  const perPage = 100;
  let page = 1;
  let total = Number.POSITIVE_INFINITY;

  while (validators.length < total) {
    const url = new URL('validators', normalizeBaseUrl(rpcBase));
    url.searchParams.set('height', String(height));
    url.searchParams.set('page', String(page));
    url.searchParams.set('per_page', String(perPage));

    const response = await fetchJson(url.toString());
    const result = response?.result ?? response;
    const pageValidators = Array.isArray(result?.validators) ? result.validators : [];
    total = Number(result?.total ?? pageValidators.length);
    validators.push(...pageValidators);

    if (pageValidators.length === 0 || validators.length >= total) {
      break;
    }
    page += 1;
  }

  return validators;
}

async function fetchValidatorMinaPubKey(
  grpcAddr,
  consensusPubKeyBytes,
  validatorMinaKeyMethod
) {
  const response = await grpcurlJson(grpcAddr, validatorMinaKeyMethod, {
    validator_cosmos_pub_key: Buffer.from(consensusPubKeyBytes).toString('base64'),
  });
  if (typeof response.validatorMinaPubKey !== 'string' || response.validatorMinaPubKey.length === 0) {
    return null;
  }
  return decodeBase64(response.validatorMinaPubKey, 'validatorMinaPubKey');
}

export async function recomputeReferenceRoot({
  grpcAddr,
  rpcBase,
  voteExtensionHeight,
  validatorMinaKeyMethod,
}) {
  if (!rpcBase) {
    throw new Error('exact attached-script verification requires --rpc to reconstruct validators');
  }

  const rpcValidators = await fetchAllValidators(rpcBase, voteExtensionHeight);
  const validators = [];
  const missingMinaKeys = [];
  const validatorsByMinaKey = new Map();

  for (const validator of rpcValidators) {
    const consensusPubKeyBase64 = validator?.pub_key?.value;
    if (typeof consensusPubKeyBase64 !== 'string' || consensusPubKeyBase64.length === 0) {
      throw new Error('validator RPC response is missing pub_key.value');
    }

    const consensusPubKeyBytes = decodeBase64(
      consensusPubKeyBase64,
      'validator.pub_key.value'
    );
    const minaPubKeyBytes = await fetchValidatorMinaPubKey(
      grpcAddr,
      consensusPubKeyBytes,
      validatorMinaKeyMethod
    );

    if (minaPubKeyBytes === null) {
      missingMinaKeys.push({
        consensusAddress: validator.address ?? null,
        consensusPubKeyBase64,
      });
      continue;
    }

    const minaPublicKey = decodeMinaPublicKey(minaPubKeyBytes);
    const minaPublicKeyBase64 = Buffer.from(minaPubKeyBytes).toString('base64');
    const power = BigInt(validator.voting_power);
    const consensusAddressBytes = decodeHex(
      validator.address,
      'validator.address'
    );

    validators.push({
      publicKey: minaPublicKey,
      power,
      consensusAddressBytes,
    });

    validatorsByMinaKey.set(minaPublicKeyBase64, {
      address: validator.address ?? null,
      consensusPubKeyBase64,
      power: power.toString(),
    });
  }

  if (missingMinaKeys.length > 0) {
    throw new Error(
      `cannot reconstruct exact validator set root: missing Mina keys for ${missingMinaKeys.length} validators`
    );
  }

  return {
    validatorCount: rpcValidators.length,
    validatorKeysResolved: validators.length,
    validators,
    validatorsByMinaKey,
  };
}
