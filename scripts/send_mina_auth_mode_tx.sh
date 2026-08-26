#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
PROJECT_NAME="${PULSAR_DOCKER_PROJECT:-pulsar-testnet-3}"
CONTAINER="${PULSAR_VALIDATOR_CONTAINER:-${PROJECT_NAME}-validator1-1}"
VALIDATOR_HOME="${PULSAR_VALIDATOR_HOME:-/testnet/.pulsar-node1}"
CHAIN_ID="${CHAIN_ID:-mytestnet}"
FROM_KEY="${AUTH_MODE_FROM_KEY:-validator1}"
RECIPIENT_KEY="${AUTH_MODE_RECIPIENT_KEY:-auth-mode-recipient}"
AMOUNT="${AUTH_MODE_SEND_AMOUNT:-1pmina}"
FEES="${AUTH_MODE_FEES:-100pmina}"
NODE="${PULSAR_NODE:-tcp://127.0.0.1:26657}"
GRPC_ADDR="${PULSAR_GRPC_ADDR:-127.0.0.1:9090}"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/pulsar-mina-auth.XXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT

docker inspect "$CONTAINER" >/dev/null

if ! docker exec "$CONTAINER" pulsard keys show "$RECIPIENT_KEY" \
  --home "$VALIDATOR_HOME" --keyring-backend test >/dev/null 2>&1; then
  docker exec "$CONTAINER" pulsard keys add "$RECIPIENT_KEY" \
    --home "$VALIDATOR_HOME" --keyring-backend test >/dev/null 2>&1
fi

FROM_ADDRESS="$(docker exec "$CONTAINER" pulsard keys show "$FROM_KEY" \
  --address --home "$VALIDATOR_HOME" --keyring-backend test)"
RECIPIENT_ADDRESS="$(docker exec "$CONTAINER" pulsard keys show "$RECIPIENT_KEY" \
  --address --home "$VALIDATOR_HOME" --keyring-backend test)"
COSMOS_PUBLIC_KEY_BASE64="$(docker exec "$CONTAINER" pulsard keys show "$FROM_KEY" \
  --pubkey --home "$VALIDATOR_HOME" --keyring-backend test \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["key"])')"
COSMOS_PUBLIC_KEY_HEX="$(python3 -c 'import base64,sys; print(base64.b64decode(sys.argv[1], validate=True).hex())' "$COSMOS_PUBLIC_KEY_BASE64")"

MINA_PRIVATE_KEY="${AUTH_MODE_MINA_PRIVATE_KEY:-$(
  python3 "$SCRIPT_DIR/setup_local_testnet_helper.py" generate-default-mina-priv-key --index auth-mode-user
)}"

cd "$REPO_ROOT"
go run -tags=purego ./scripts/devtools mina-registration-material \
  --chain-id "$CHAIN_ID" \
  --cosmos-public-key "$COSMOS_PUBLIC_KEY_BASE64" \
  --mina-private-key "$MINA_PRIVATE_KEY" \
  >"$WORK_DIR/mina-material.json"

MINA_PUBLIC_KEY_HEX="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mina_public_key"])' "$WORK_DIR/mina-material.json")"
MINA_SIGNATURE_HEX="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["mina_signature"])' "$WORK_DIR/mina-material.json")"

if docker exec "$CONTAINER" pulsard query keyregistry get-user-mina-public-key \
  "$COSMOS_PUBLIC_KEY_HEX" --node "$NODE" --output json \
  >"$WORK_DIR/registered-key.json" 2>/dev/null; then
  REGISTERED_MINA_KEY_HEX="$(python3 -c 'import base64,json,sys; print(base64.b64decode(json.load(open(sys.argv[1]))["user_mina_public_key"]).hex())' "$WORK_DIR/registered-key.json")"
  if [[ "$REGISTERED_MINA_KEY_HEX" != "$MINA_PUBLIC_KEY_HEX" ]]; then
    echo "error: $FROM_KEY already has a different Mina key registered" >&2
    exit 1
  fi
else
  docker exec "$CONTAINER" pulsard tx keyregistry register-user-keys \
    "$COSMOS_PUBLIC_KEY_HEX" "$MINA_PUBLIC_KEY_HEX" "$MINA_SIGNATURE_HEX" \
    --from "$FROM_KEY" \
    --home "$VALIDATOR_HOME" \
    --keyring-backend test \
    --chain-id "$CHAIN_ID" \
    --node "$NODE" \
    --gas 250000 \
    --fees "$FEES" \
    --yes \
    --output json >"$WORK_DIR/register-broadcast.json"

  REGISTER_TX_HASH="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["txhash"])' "$WORK_DIR/register-broadcast.json")"
  for _ in $(seq 1 60); do
    if docker exec "$CONTAINER" pulsard query tx "$REGISTER_TX_HASH" --node "$NODE" --output json \
      >"$WORK_DIR/register-result.json" 2>/dev/null; then
      REGISTER_CODE="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["code"])' "$WORK_DIR/register-result.json")"
      if [[ "$REGISTER_CODE" != "0" ]]; then
        cat "$WORK_DIR/register-result.json" >&2
        exit 1
      fi
      break
    fi
    sleep 1
  done
  [[ -f "$WORK_DIR/register-result.json" ]] || {
    echo "error: Mina key registration transaction was not included: $REGISTER_TX_HASH" >&2
    exit 1
  }
fi

go run -tags=purego ./scripts/devtools send-mina-auth-mode-tx \
  --grpc-addr "$GRPC_ADDR" \
  --chain-id "$CHAIN_ID" \
  --from "$FROM_ADDRESS" \
  --to "$RECIPIENT_ADDRESS" \
  --mina-private-key "$MINA_PRIVATE_KEY" \
  --amount "$AMOUNT" \
  --fees "$FEES"
