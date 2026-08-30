#!/usr/bin/env bash

set -Eeuo pipefail

PROJECT_NAME="${PULSAR_DOCKER_PROJECT:-pulsar-testnet-3}"
CONTAINER="${PULSAR_VALIDATOR_CONTAINER:-${PROJECT_NAME}-validator1-1}"
VALIDATOR_HOME="${PULSAR_VALIDATOR_HOME:-/testnet/.pulsar-node1}"
CHAIN_ID="${CHAIN_ID:-mytestnet}"
FROM_KEY="${AUTH_MODE_FROM_KEY:-validator1}"
RECIPIENT_KEY="${AUTH_MODE_RECIPIENT_KEY:-auth-mode-recipient}"
AMOUNT="${AUTH_MODE_SEND_AMOUNT:-1pmina}"
FEES="${AUTH_MODE_FEES:-100pmina}"
NODE="${PULSAR_NODE:-tcp://127.0.0.1:26657}"
MODE="TX_AUTH_MODE_UNSPECIFIED"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/pulsar-unspecified-auth.XXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT

docker inspect "$CONTAINER" >/dev/null

if ! docker exec "$CONTAINER" pulsard keys show "$RECIPIENT_KEY" \
  --home "$VALIDATOR_HOME" --keyring-backend test >/dev/null 2>&1; then
  docker exec "$CONTAINER" pulsard keys add "$RECIPIENT_KEY" \
    --home "$VALIDATOR_HOME" --keyring-backend test >/dev/null 2>&1
fi

RECIPIENT_ADDRESS="$(docker exec "$CONTAINER" pulsard keys show "$RECIPIENT_KEY" \
  --address --home "$VALIDATOR_HOME" --keyring-backend test)"

docker exec "$CONTAINER" pulsard tx bank send "$FROM_KEY" "$RECIPIENT_ADDRESS" "$AMOUNT" \
  --from "$FROM_KEY" \
  --home "$VALIDATOR_HOME" \
  --keyring-backend test \
  --chain-id "$CHAIN_ID" \
  --node "$NODE" \
  --gas 200000 \
  --fees "$FEES" \
  --generate-only \
  --output json >"$WORK_DIR/unsigned.json"

python3 - "$WORK_DIR/unsigned.json" "$MODE" >"$WORK_DIR/extended.json" <<'PY'
import json
import sys

tx = json.load(open(sys.argv[1], encoding="utf-8"))
tx["body"]["extension_options"] = [{
    "@type": "/pulsarchain.ante.v1.TxAuthModeExtension",
    "tx_auth_mode": sys.argv[2],
}]
json.dump(tx, sys.stdout)
PY

docker exec -i "$CONTAINER" pulsard tx sign /dev/stdin \
  --from "$FROM_KEY" \
  --home "$VALIDATOR_HOME" \
  --keyring-backend test \
  --chain-id "$CHAIN_ID" \
  --node "$NODE" \
  --output-document /dev/stdout \
  <"$WORK_DIR/extended.json" >"$WORK_DIR/signed.json"

docker exec -i "$CONTAINER" pulsard tx broadcast /dev/stdin \
  --node "$NODE" --output json \
  <"$WORK_DIR/signed.json" >"$WORK_DIR/broadcast.json"

TX_HASH="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["txhash"])' "$WORK_DIR/broadcast.json")"
for _ in $(seq 1 60); do
  if docker exec "$CONTAINER" pulsard query tx "$TX_HASH" --node "$NODE" --output json \
    >"$WORK_DIR/result.json" 2>/dev/null; then
    CODE="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["code"])' "$WORK_DIR/result.json")"
    cat "$WORK_DIR/result.json"
    [[ "$CODE" == "0" ]]
    exit
  fi
  sleep 1
done

echo "error: transaction was not included: $TX_HASH" >&2
exit 1
