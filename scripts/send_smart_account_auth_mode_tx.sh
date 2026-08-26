#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
PROJECT_NAME="${PULSAR_DOCKER_PROJECT:-pulsar-testnet-3}"
CONTAINER="${PULSAR_VALIDATOR_CONTAINER:-${PROJECT_NAME}-validator1-1}"
VALIDATOR_HOME="${PULSAR_VALIDATOR_HOME:-/testnet/.pulsar-node1}"
CHAIN_ID="${CHAIN_ID:-mytestnet}"
OWNER_KEY="${AUTH_MODE_FROM_KEY:-validator1}"
RECIPIENT_KEY="${AUTH_MODE_RECIPIENT_KEY:-auth-mode-recipient}"
IDENTITY_HEX="${SMART_ACCOUNT_AUTH_MODE_IDENTITY_HEX:-a54bcb4b2f5749e85d7a4038e291882da23adfa36d2f8ad374d1847dc6eeb4ec}"
SESSION_PRIVATE_KEY="${SMART_ACCOUNT_AUTH_MODE_SESSION_PRIVATE_KEY:-517a96fb99ad7a1af9f2064c7f87738a5059c527b18c5ed6898f7b583862684c}"
AMOUNT="${AUTH_MODE_SEND_AMOUNT:-1pmina}"
FEES="${AUTH_MODE_FEES:-100pmina}"
GRPC_ADDR="${PULSAR_GRPC_ADDR:-127.0.0.1:9090}"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/pulsar-smart-account-auth.XXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT

docker inspect "$CONTAINER" >/dev/null

if ! docker exec "$CONTAINER" pulsard keys show "$RECIPIENT_KEY" \
  --home "$VALIDATOR_HOME" --keyring-backend test >/dev/null 2>&1; then
  docker exec "$CONTAINER" pulsard keys add "$RECIPIENT_KEY" \
    --home "$VALIDATOR_HOME" --keyring-backend test >/dev/null 2>&1
fi

OWNER_ADDRESS="$(docker exec "$CONTAINER" pulsard keys show "$OWNER_KEY" \
  --address --home "$VALIDATOR_HOME" --keyring-backend test)"
RECIPIENT_ADDRESS="$(docker exec "$CONTAINER" pulsard keys show "$RECIPIENT_KEY" \
  --address --home "$VALIDATOR_HOME" --keyring-backend test)"

printf '%s\n' "$SESSION_PRIVATE_KEY" >"$WORK_DIR/session-key.hex"

cd "$REPO_ROOT"
go run -tags=purego ./scripts/devtools send-smart-account-tx \
  --grpc-addr "$GRPC_ADDR" \
  --chain-id "$CHAIN_ID" \
  --from "$OWNER_ADDRESS" \
  --to "$RECIPIENT_ADDRESS" \
  --identity "$IDENTITY_HEX" \
  --session-key-file "$WORK_DIR/session-key.hex" \
  --amount "$AMOUNT" \
  --fees "$FEES"
