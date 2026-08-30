#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
PROJECT_NAME="${PULSAR_DOCKER_PROJECT:-pulsar-testnet-3}"
CONTAINER="${PULSAR_VALIDATOR_CONTAINER:-${PROJECT_NAME}-validator1-1}"
VERIFIER_CONTAINER="${PULSAR_VERIFIER_CONTAINER:-${PROJECT_NAME}-verifier1-1}"
VALIDATOR_HOME="${PULSAR_VALIDATOR_HOME:-/testnet/.pulsar-node1}"
OWNER_KEY="${SMART_ACCOUNT_OWNER_KEY:-smart-account-owner}"
FIXTURE_GENERATOR="$SCRIPT_DIR/generate_smart_account_noir_fixture.sh"
SUBMISSION_PROTO_ROOT="$REPO_ROOT/testdata/smartaccounts/verifier-proto"
CHAIN_ID="${CHAIN_ID:-mytestnet}"
AMOUNT="${SMART_ACCOUNT_SEND_AMOUNT:-1pmina}"
FUND_AMOUNT="${SMART_ACCOUNT_FUND_AMOUNT:-1000000pmina}"
FEES="${SMART_ACCOUNT_FEES:-100pmina}"
GRPC_ADDR="${PULSAR_GRPC_ADDR:-127.0.0.1:9090}"
GRPCURL_IMAGE="${GRPCURL_IMAGE:-fullstorydev/grpcurl@sha256:085e183ca334eb4e81ca81ee12cbb2b2737505d1d77f5e33dabc5d066593d998}"
SESSION_PRIVATE_KEY="${SMART_ACCOUNT_SESSION_PRIVATE_KEY:-8a5eed4d959d619565a87e8193ad3c91abbce9d783497c004c1b08606c3d9b6b}"
IDENTITY_HEX="${SMART_ACCOUNT_IDENTITY_HEX:-2f833b7c56ed097e355807c19173647fd36cae0f66546b6d3c356a821d557e38}"
EXPIRATION_BLOCKS="${SMART_ACCOUNT_EXPIRATION_BLOCKS:-10000}"

usage() {
  cat <<'EOF'
usage: scripts/send_smart_account_tx.sh

Runs the complete local smart-account end-to-end flow:
  1. generates a real UltraHonk proof with the pinned Nargo/Barretenberg tools
  2. submits the proof through the production verifier sidecar
  3. waits for the chain to finalize the proof as VALID
  4. funds the proof-bound Cosmos owner
  5. registers the proof-bound session key with AddPublicKey
  6. sends a bank transaction authenticated by that session key

The chain must be deployed with verifier sidecars. The smartaccounts
verification_key_hash in config.yml must equal SHA-256 of this repository's
test-only smart-account circuit verification key.

Optional environment variables:
  PULSAR_DOCKER_PROJECT        Compose project (default: pulsar-testnet-3)
  PULSAR_VALIDATOR_CONTAINER  validator1 container override
  PULSAR_VERIFIER_CONTAINER   verifier1 container override
  SMART_ACCOUNT_SEND_AMOUNT   final smart-account transfer (default: 1pmina)
  SMART_ACCOUNT_FUND_AMOUNT   owner funding amount (default: 1000000pmina)
  SMART_ACCOUNT_FEES          fee per transaction (default: 100pmina)
  SMART_ACCOUNT_EXPIRATION_BLOCKS
                              session-key lifetime (default: 10000 blocks)
  SMART_ACCOUNT_SESSION_PRIVATE_KEY
                              32-byte field-compatible Ed25519 seed in hex
  SMART_ACCOUNT_IDENTITY_HEX  32-byte field-compatible identity in hex
  GRPCURL_IMAGE               grpcurl container image
EOF
}

case "${1:-}" in
  -h | --help)
    usage
    exit 0
    ;;
  "") ;;
  *)
    usage >&2
    exit 2
    ;;
esac

fail() {
  echo "error: $*" >&2
  exit 1
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "missing required command: $1"
  fi
}

json_value() {
  python3 -c 'import json,sys
value=json.load(open(sys.argv[1], encoding="utf-8"))
for key in sys.argv[2].split("."):
    value=value[key]
print(value)' "$1" "$2"
}

sha256_file() {
  python3 -c 'import hashlib,pathlib,sys
print(hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest())' "$1"
}

wait_for_tx() {
  local tx_hash="$1"
  local output="$2"

  for _ in $(seq 1 60); do
    if docker exec "$CONTAINER" pulsard query tx "$tx_hash" \
      --node tcp://127.0.0.1:26657 \
      --output json >"$output" 2>/dev/null; then
      local code
      code="$(json_value "$output" code)"
      if [[ "$code" != "0" ]]; then
        echo "transaction $tx_hash failed:" >&2
        cat "$output" >&2
        return 1
      fi
      return 0
    fi
    sleep 1
  done

  echo "transaction was not included: $tx_hash" >&2
  return 1
}

require_cmd docker
require_cmd go
require_cmd python3

[[ -x "$FIXTURE_GENERATOR" ]] || fail "fixture generator is not executable: $FIXTURE_GENERATOR"
if ! [[ "$EXPIRATION_BLOCKS" =~ ^[0-9]+$ ]] || (( EXPIRATION_BLOCKS < 1 )); then
  fail "SMART_ACCOUNT_EXPIRATION_BLOCKS must be a positive integer"
fi

SUBMISSION_PROTO="$SUBMISSION_PROTO_ROOT/pulsar/verifier/v1/submission_service.proto"
[[ -f "$SUBMISSION_PROTO" ]] || fail "submission-service proto not found: $SUBMISSION_PROTO"

docker inspect "$CONTAINER" >/dev/null 2>&1 || \
  fail "validator container not found: $CONTAINER"
docker inspect "$VERIFIER_CONTAINER" >/dev/null 2>&1 || \
  fail "verifier container not found: $VERIFIER_CONTAINER; deploy with PULSAR_VERIFIER_IMAGE set"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/pulsar-smart-account.XXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT
FIXTURE_DIR="$WORK_DIR/fixture"

if ! docker exec "$CONTAINER" pulsard keys show "$OWNER_KEY" \
  --home "$VALIDATOR_HOME" \
  --keyring-backend test >/dev/null 2>&1; then
  docker exec "$CONTAINER" pulsard keys add "$OWNER_KEY" \
    --home "$VALIDATOR_HOME" \
    --keyring-backend test >/dev/null 2>&1
fi

OWNER_ADDRESS="$(docker exec "$CONTAINER" pulsard keys show "$OWNER_KEY" \
  --address --home "$VALIDATOR_HOME" --keyring-backend test)"
VALIDATOR_ADDRESS="$(docker exec "$CONTAINER" pulsard keys show validator1 \
  --address --home "$VALIDATOR_HOME" --keyring-backend test)"
OWNER_ADDRESS_HEX="$(docker exec "$CONTAINER" pulsard debug addr "$OWNER_ADDRESS" \
  | sed -n 's/^Address (hex): //p' | tr '[:upper:]' '[:lower:]')"
[[ "$OWNER_ADDRESS_HEX" =~ ^[0-9a-f]{40}$ ]] || \
  fail "could not derive the 20-byte canonical address for $OWNER_ADDRESS"

CURRENT_HEIGHT="$(docker exec "$CONTAINER" pulsard status 2>/dev/null \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["sync_info"]["latest_block_height"])')"
EXPIRES_AT_HEIGHT="$((CURRENT_HEIGHT + EXPIRATION_BLOCKS))"

SESSION_PUBLIC_KEY_HEX="$(
  cd "$REPO_ROOT"
  go run -tags=purego ./scripts/devtools derive-ed25519-pub "$SESSION_PRIVATE_KEY"
)"

echo "==> Generating the smart-account Noir proof"
"$FIXTURE_GENERATOR" \
  --output-dir "$FIXTURE_DIR" \
  --session-private-key "$SESSION_PRIVATE_KEY" \
  --session-public-key "$SESSION_PUBLIC_KEY_HEX" \
  --expires-at-height "$EXPIRES_AT_HEIGHT" \
  --identity "$IDENTITY_HEX" \
  --account-address "$OWNER_ADDRESS_HEX"

PROOF_HASH="$(sha256_file "$FIXTURE_DIR/proof")"
PUBLIC_INPUTS_HASH="$(sha256_file "$FIXTURE_DIR/public_inputs")"
VERIFICATION_KEY_HASH="$(sha256_file "$FIXTURE_DIR/vk")"
VERIFICATION_ID_HEX="$(python3 - "$PROOF_HASH" "$PUBLIC_INPUTS_HASH" "$VERIFICATION_KEY_HASH" <<'PY'
import hashlib
import sys

proof_hash, public_inputs_hash, verification_key_hash = map(bytes.fromhex, sys.argv[1:])
payload = (
    b"pulsar/verification/v1\x00"
    + (2).to_bytes(4, byteorder="big")
    + proof_hash
    + public_inputs_hash
    + verification_key_hash
)
print(hashlib.sha256(payload).hexdigest())
PY
)"

docker exec "$CONTAINER" pulsard query smartaccounts params \
  --node tcp://127.0.0.1:26657 \
  --output json >"$WORK_DIR/smartaccounts-params.json"
CHAIN_VERIFICATION_KEY_HASH="$(python3 - "$WORK_DIR/smartaccounts-params.json" <<'PY'
import base64
import json
import sys

value = json.load(open(sys.argv[1], encoding="utf-8"))["params"]["verification_key_hash"]
print(base64.b64decode(value, validate=True).hex())
PY
)"
if [[ "$CHAIN_VERIFICATION_KEY_HASH" != "$VERIFICATION_KEY_HASH" ]]; then
  fail "smartaccounts verification_key_hash does not match SHA-256(vk); redeploy with SMART_ACCOUNTS_VERIFICATION_KEY_HASH=$VERIFICATION_KEY_HASH"
fi

echo "==> Submitting the real Noir smart-account proof through verifier1"
TX_RAW_BASE64="$(docker exec "$CONTAINER" bash -ceu '
  work="$(mktemp -d)"
  trap '\''rm -rf "$work"'\'' EXIT
  pulsard tx verification submit-proof "$1" noir-barretenberg "$2" "$3" \
    --from validator1 \
    --home "$4" \
    --keyring-backend test \
    --chain-id "$5" \
    --fees "$6" \
    --generate-only \
    --output json >"$work/unsigned.json"
  pulsard tx sign "$work/unsigned.json" \
    --from validator1 \
    --home "$4" \
    --keyring-backend test \
    --chain-id "$5" \
    --output-document "$work/signed.json"
  pulsard tx encode "$work/signed.json"
' -- "$PROOF_HASH" "$PUBLIC_INPUTS_HASH" "$VERIFICATION_KEY_HASH" "$VALIDATOR_HOME" "$CHAIN_ID" "$FEES")"

python3 - "$FIXTURE_DIR" "$TX_RAW_BASE64" >"$WORK_DIR/submission-request.json" <<'PY'
import base64
import json
import pathlib
import sys

fixture = pathlib.Path(sys.argv[1])
payload = {
    "proof": {
        "proofType": "PROOF_TYPE_NOIR_BARRETENBERG",
        "proof": base64.b64encode((fixture / "proof").read_bytes()).decode(),
        "publicInputs": base64.b64encode((fixture / "public_inputs").read_bytes()).decode(),
        "verificationKey": base64.b64encode((fixture / "vk").read_bytes()).decode(),
    },
    "txRaw": sys.argv[2].strip(),
}
json.dump(payload, sys.stdout)
PY

docker run --rm -i \
  --network "container:${VERIFIER_CONTAINER}" \
  --mount "type=bind,src=$SUBMISSION_PROTO_ROOT,dst=/proto,readonly" \
  "$GRPCURL_IMAGE" -plaintext \
  -max-time 30 \
  -import-path /proto \
  -proto pulsar/verifier/v1/submission_service.proto \
  -d @ 127.0.0.1:50052 \
  pulsar.verifier.v1.SubmissionService/SubmitProof \
  <"$WORK_DIR/submission-request.json" >"$WORK_DIR/submission-response.json"

RESPONSE_VERIFICATION_ID_HEX="$(python3 - "$WORK_DIR/submission-response.json" <<'PY'
import base64
import json
import sys

response = json.load(open(sys.argv[1], encoding="utf-8"))
print(base64.b64decode(response["verificationId"], validate=True).hex())
PY
)"
if [[ "$RESPONSE_VERIFICATION_ID_HEX" != "$VERIFICATION_ID_HEX" ]]; then
  fail "sidecar returned an unexpected verification ID: $RESPONSE_VERIFICATION_ID_HEX"
fi

VERIFICATION_TX_HASH="$(python3 - "$WORK_DIR/submission-response.json" <<'PY'
import base64
import json
import sys

response = json.load(open(sys.argv[1], encoding="utf-8"))
print(base64.b64decode(response["transactionHash"], validate=True).hex().upper())
PY
)"
wait_for_tx "$VERIFICATION_TX_HASH" "$WORK_DIR/verification-tx-result.json"

echo "==> Waiting for proof finalization: $VERIFICATION_ID_HEX"
VERIFICATION_STATUS=""
for _ in $(seq 1 90); do
  if docker exec "$CONTAINER" pulsard query verification proof-by-verification-id \
    "$VERIFICATION_ID_HEX" \
    --node tcp://127.0.0.1:26657 \
    --output json >"$WORK_DIR/verification-result.json" 2>/dev/null; then
    VERIFICATION_STATUS="$(python3 - "$WORK_DIR/verification-result.json" <<'PY'
import json
import sys

def find_status(value):
    if isinstance(value, dict):
        if "status" in value:
            return value["status"]
        for child in value.values():
            found = find_status(child)
            if found is not None:
                return found
    elif isinstance(value, list):
        for child in value:
            found = find_status(child)
            if found is not None:
                return found
    return None

status = find_status(json.load(open(sys.argv[1], encoding="utf-8")))
print(status or "")
PY
)"
    [[ -n "$VERIFICATION_STATUS" ]] && break
  fi
  sleep 2
done

[[ -n "$VERIFICATION_STATUS" ]] || fail "proof did not finalize before timeout: $VERIFICATION_ID_HEX"
[[ "$VERIFICATION_STATUS" == "PROOF_STATUS_VALID" ]] || \
  fail "proof finalized with status $VERIFICATION_STATUS"

echo "==> Funding proof-bound smart-account owner: $OWNER_ADDRESS"
docker exec "$CONTAINER" pulsard tx bank send validator1 "$OWNER_ADDRESS" "$FUND_AMOUNT" \
  --from validator1 \
  --home "$VALIDATOR_HOME" \
  --keyring-backend test \
  --chain-id "$CHAIN_ID" \
  --node tcp://127.0.0.1:26657 \
  --gas 200000 \
  --fees "$FEES" \
  --yes \
  --output json >"$WORK_DIR/fund-broadcast.json"
FUND_TX_HASH="$(json_value "$WORK_DIR/fund-broadcast.json" txhash)"
wait_for_tx "$FUND_TX_HASH" "$WORK_DIR/fund-result.json"

echo "==> Registering the proof-bound session key"
docker exec "$CONTAINER" pulsard tx smartaccounts add-public-key \
  "$VERIFICATION_ID_HEX" "$SESSION_PUBLIC_KEY_HEX" "$EXPIRES_AT_HEIGHT" "$IDENTITY_HEX" \
  --from "$OWNER_KEY" \
  --home "$VALIDATOR_HOME" \
  --keyring-backend test \
  --chain-id "$CHAIN_ID" \
  --node tcp://127.0.0.1:26657 \
  --gas 300000 \
  --fees "$FEES" \
  --yes \
  --output json >"$WORK_DIR/add-key-broadcast.json"
ADD_KEY_TX_HASH="$(json_value "$WORK_DIR/add-key-broadcast.json" txhash)"
wait_for_tx "$ADD_KEY_TX_HASH" "$WORK_DIR/add-key-result.json"

echo "==> Sending $AMOUNT with the smart-account session key"
cd "$REPO_ROOT"
go run -tags=purego ./scripts/devtools send-smart-account-tx \
  --grpc-addr "$GRPC_ADDR" \
  --chain-id "$CHAIN_ID" \
  --from "$OWNER_ADDRESS" \
  --to "$VALIDATOR_ADDRESS" \
  --identity "$IDENTITY_HEX" \
  --session-key-file "$FIXTURE_DIR/session-key.hex" \
  --amount "$AMOUNT" \
  --fees "$FEES"
