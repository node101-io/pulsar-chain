#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
WRAPPER_SOURCE="${ARCHIVE_WRAPPER_SOURCE:-$(cd -- "$REPO_ROOT/../archive-wrapper" && pwd)}"
VERIFIER_SOURCE="${PULSAR_VERIFIER_SOURCE:-$(cd -- "$REPO_ROOT/../../rust-workspace/pulsar-verifier" && pwd)}"
EXPECTED_WRAPPER_SHA="cd42a203ac6b43d24d9fbd57c323ecd52ea52bd5"
ENABLE_VERIFIER_SIDECARS="${ENABLE_VERIFIER_SIDECARS:-0}"
MODE="${1:-shared}"
PROJECT="pulsar-wrapper-e2e-${MODE//[^a-zA-Z0-9]/-}-$$"
TMP_DIR="$(mktemp -d)"
DOCKER_STATE_ROOT="$TMP_DIR/docker-state"
GENERATED_ROOT="$DOCKER_STATE_ROOT/$PROJECT"
COMPOSE_FILE="$GENERATED_ROOT/compose.json"
POSTGRES_COMPOSE_FILE="$TMP_DIR/postgres.json"
GENERATED_DIR="$GENERATED_ROOT/wrapper-configs"
SEED_FILE="$TMP_DIR/seed.sql"
PULSAR_IMAGE="pulsar-chain:e2e-$(git -C "$REPO_ROOT" rev-parse --short=12 HEAD)"
WRAPPER_IMAGE="archive-wrapper:e2e-${EXPECTED_WRAPPER_SHA:0:12}"
VERIFIER_IMAGE="pulsar-verifier:e2e-$(git -C "$VERIFIER_SOURCE" rev-parse --short=12 HEAD)"
E2E_USER_MINA_PRIV_KEY="AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAE="
POSTGRES_URI="postgres://archive:e2e-secret@postgres:5432/archive?sslmode=disable"
VALIDATOR_COUNT=3
COMPOSE_STARTED=0
EXTERNAL_WRAPPER_NAME="${PROJECT}-external-wrapper"
EXTERNAL_NETWORK="${PROJECT}-external"
EXTERNAL_DATA_VOLUME="${PROJECT}-external-wrapper-data"
EXTERNAL_WRAPPER_STARTED=0
EXTERNAL_NETWORK_CREATED=0
ARTIFACT_DIR="${E2E_ARTIFACT_DIR:-}"
PORT_BASE="$((30000 + ($$ % 10000)))"

declare -a HOST_RPC_PORTS HOST_GRPC_PORTS
for index in 1 2 3; do
  HOST_RPC_PORTS[index]="$((PORT_BASE + index - 1))"
  HOST_GRPC_PORTS[index]="$((PORT_BASE + 100 + index - 1))"
  printf -v "VALIDATOR${index}_HOST_RPC_PORT" '%s' "${HOST_RPC_PORTS[index]}"
  printf -v "VALIDATOR${index}_HOST_GRPC_PORT" '%s' "${HOST_GRPC_PORTS[index]}"
  printf -v "VALIDATOR${index}_HOST_API_PORT" '%s' "$((PORT_BASE + 200 + index - 1))"
  printf -v "VALIDATOR${index}_HOST_PPROF_PORT" '%s' "$((PORT_BASE + 300 + index - 1))"
  export "VALIDATOR${index}_HOST_RPC_PORT"
  export "VALIDATOR${index}_HOST_GRPC_PORT"
  export "VALIDATOR${index}_HOST_API_PORT"
  export "VALIDATOR${index}_HOST_PPROF_PORT"
done

compose() {
  docker compose \
    --project-name "$PROJECT" \
    -f "$COMPOSE_FILE" \
    -f "$POSTGRES_COMPOSE_FILE" \
    "$@"
}

redact_output() {
  sed -E \
    -e 's#postgres://[^ @]+@#postgres://[REDACTED]@#g' \
    -e 's/e2e-secret/[REDACTED]/g'
}

capture_failure_artifacts() {
  [[ -n "$ARTIFACT_DIR" ]] || return 0

  mkdir -p "$ARTIFACT_DIR"
  compose ps -a >"$ARTIFACT_DIR/${MODE}-compose-ps.txt" 2>&1 || true
  compose logs --no-color 2>&1 \
    | redact_output >"$ARTIFACT_DIR/${MODE}-compose.log" || true
  docker image inspect \
    --format '{{json .Id}} {{json .RepoTags}} {{json .Config.Labels}}' \
    "$PULSAR_IMAGE" "$WRAPPER_IMAGE" \
    >"$ARTIFACT_DIR/${MODE}-image-metadata.txt" 2>&1 || true
  if (( EXTERNAL_WRAPPER_STARTED == 1 )); then
    docker logs "$EXTERNAL_WRAPPER_NAME" 2>&1 \
      | redact_output >"$ARTIFACT_DIR/${MODE}-external-wrapper.log" || true
  fi

  for artifact in "$TMP_DIR"/wrapper-query-*.json "$TMP_DIR"/tx-result.json \
    "$TMP_DIR"/verification-*.json "$TMP_DIR"/root-*.json "$TMP_DIR"/block-*.json; do
    [[ -f "$artifact" ]] && cp "$artifact" "$ARTIFACT_DIR/${MODE}-$(basename "$artifact")"
  done
}

cleanup() {
  local status=$?
  trap - EXIT INT TERM
  if (( status != 0 && COMPOSE_STARTED == 1 )); then
    capture_failure_artifacts
    compose ps -a >&2 || true
    compose logs --no-color 2>&1 | redact_output >&2 || true
  fi
  if (( EXTERNAL_WRAPPER_STARTED == 1 )); then
    docker rm -f "$EXTERNAL_WRAPPER_NAME" >/dev/null 2>&1 || true
  fi
  if (( COMPOSE_STARTED == 1 )); then
    compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  fi
  docker volume rm "$EXTERNAL_DATA_VOLUME" >/dev/null 2>&1 || true
  if (( EXTERNAL_NETWORK_CREATED == 1 )); then
    docker network rm "$EXTERNAL_NETWORK" >/dev/null 2>&1 || true
  fi
  rm -rf "$TMP_DIR"
  exit "$status"
}
trap cleanup EXIT INT TERM

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

wait_for_tx() {
  local tx_hash="$1"
  local output="$2"
  for _ in $(seq 1 60); do
    if compose exec -T validator1 pulsard query tx "$tx_hash" \
      --node tcp://127.0.0.1:26657 --output json >"$output" 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "transaction was not included: $tx_hash" >&2
  return 1
}

wait_for_height() {
	local target_height="$1"
	local latest_height
	for _ in $(seq 1 60); do
		latest_height="$(curl -fsS "http://127.0.0.1:${HOST_RPC_PORTS[1]}/status" 2>/dev/null \
			| python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value --input - --path result.sync_info.latest_block_height || true)"
		if [[ "$latest_height" =~ ^[0-9]+$ ]] && (( latest_height >= target_height )); then
			return 0
		fi
		sleep 1
	done
	echo "chain did not reach height $target_height" >&2
	return 1
}

validator_health() {
  compose exec -T "$1" /opt/pulsar/scripts/docker_entrypoint.sh healthcheck-validator
}

wait_for_container_health() {
  local container="$1"
  for _ in $(seq 1 60); do
    if [[ "$(docker inspect --format '{{.State.Health.Status}}' "$container" 2>/dev/null || true)" == "healthy" ]]; then
      return 0
    fi
    sleep 2
  done
  echo "container did not become healthy: $container" >&2
  return 1
}

run_verifier_e2e() {
  local fixture="$VERIFIER_SOURCE/tests/fixtures/noir/bb-5.2.0"
  local proof_hash public_inputs_hash verification_key_hash tx_raw_b64 verifier_container

  for index in 1 2 3; do
    compose exec -T "validator${index}" grep -A8 '^\[verification\]$' \
      "/testnet/.pulsar-node${index}/config/app.toml" | grep -q '^enabled = true$'
  done

  proof_hash="$(sha256sum "$fixture/proof" | cut -d' ' -f1)"
  public_inputs_hash="$(sha256sum "$fixture/public_inputs" | cut -d' ' -f1)"
  verification_key_hash="$(sha256sum "$fixture/vk" | cut -d' ' -f1)"
  tx_raw_b64="$(compose exec -T validator1 bash -ceu '
    work="$(mktemp -d)"
    trap '\''rm -rf "$work"'\'' EXIT
    pulsard tx verification submit-proof "$1" noir-barretenberg "$2" "$3" \
      --from validator1 \
      --home /testnet/.pulsar-node1 \
      --keyring-backend test \
      --chain-id mytestnet \
      --fees 0pmina \
      --generate-only \
      --output json >"$work/unsigned.json"
    pulsard tx sign "$work/unsigned.json" \
      --from validator1 \
      --home /testnet/.pulsar-node1 \
      --keyring-backend test \
      --chain-id mytestnet \
      --output-document "$work/signed.json"
    pulsard tx encode "$work/signed.json"
  ' -- "$proof_hash" "$public_inputs_hash" "$verification_key_hash")"

  python3 - "$fixture" "$tx_raw_b64" >"$TMP_DIR/submission-request.json" <<'PY'
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

  verifier_container="$(compose ps -q verifier1)"
  docker run --rm -i \
    --network "container:${verifier_container}" \
    --entrypoint /usr/local/bin/grpcurl \
    --mount "type=bind,src=$(command -v grpcurl),dst=/usr/local/bin/grpcurl,readonly" \
    --mount "type=bind,src=$VERIFIER_SOURCE/crates/pulsar-verifier-proto/proto,dst=/proto,readonly" \
    "$VERIFIER_IMAGE" -plaintext \
    -import-path /proto \
    -proto pulsar/verifier/v1/submission_service.proto \
    -d @ 127.0.0.1:50052 \
    pulsar.verifier.v1.SubmissionService/SubmitProof \
    <"$TMP_DIR/submission-request.json" >"$TMP_DIR/submission-response.json"

  VERIFICATION_TX_HASH="$(python3 - "$TMP_DIR/submission-response.json" <<'PY'
import base64
import json
import sys

print(base64.b64decode(json.load(open(sys.argv[1]))["transactionHash"]).hex().upper())
PY
)"
  wait_for_tx "$VERIFICATION_TX_HASH" "$TMP_DIR/verification-tx-result.json"
  VERIFICATION_PROOF_HEIGHT="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
    --input "$TMP_DIR/verification-tx-result.json" --path height)"
  VERIFICATION_FINAL_HEIGHT="$((VERIFICATION_PROOF_HEIGHT + 5))"
  wait_for_height "$VERIFICATION_FINAL_HEIGHT"

  compose exec -T validator1 pulsard query verification final-proof-result \
    "$VERIFICATION_PROOF_HEIGHT" 0 \
    --node tcp://127.0.0.1:26657 --output json >"$TMP_DIR/verification-final.json"
  VERIFICATION_STATUS="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
    --input "$TMP_DIR/verification-final.json" --path final_result.status)"
  VALID_POWER="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
    --input "$TMP_DIR/verification-final.json" --path final_result.valid_voting_power --default 0)"
  TOTAL_POWER="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
    --input "$TMP_DIR/verification-final.json" --path final_result.total_voting_power)"
  POWER_THRESHOLD="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
    --input "$TMP_DIR/verification-final.json" --path final_result.voting_power_threshold)"

  [[ "$VERIFICATION_STATUS" == "PROOF_STATUS_VALID" ]]
  [[ "$VALID_POWER" == "$TOTAL_POWER" ]]
  (( POWER_THRESHOLD == TOTAL_POWER * 2 / 3 + 1 ))

  echo "${MODE} verifier E2E evidence: proof_height=${VERIFICATION_PROOF_HEIGHT} final_height=${VERIFICATION_FINAL_HEIGHT} status=${VERIFICATION_STATUS} valid_power=${VALID_POWER} total_power=${TOTAL_POWER} threshold=${POWER_THRESHOLD} ingress=verifier1 retrieval=verifier2,verifier3"
}

verify_grpc_bind_failure() {
  local output="$TMP_DIR/grpc-bind-failure.log"
  local status

  set +e
  timeout 30s docker compose \
    --project-name "$PROJECT" \
    -f "$COMPOSE_FILE" \
    -f "$POSTGRES_COMPOSE_FILE" \
    run -T --rm --no-deps --entrypoint /bin/bash validator1 -ceu '
      python3 -m http.server 9090 >/tmp/grpc-port-holder.log 2>&1 &
      holder=$!
      trap '\''kill "$holder" >/dev/null 2>&1 || true'\'' EXIT
      sleep 1
      if ! kill -0 "$holder" >/dev/null 2>&1; then
        cat /tmp/grpc-port-holder.log >&2
        exit 1
      fi
      set +e
      pulsard start --home "$VALIDATOR_HOME" --grpc.address 127.0.0.1:9090
      status=$?
      set -e
      kill "$holder" >/dev/null 2>&1 || true
      wait "$holder" >/dev/null 2>&1 || true
      trap - EXIT
      exit "$status"
    ' >"$output" 2>&1
  status=$?
  set -e

  if (( status == 0 )); then
    echo "pulsard start unexpectedly succeeded while its gRPC port was occupied" >&2
    return 1
  fi
  if (( status == 124 )); then
    echo "pulsard start did not exit after its gRPC listener failed" >&2
    cat "$output" >&2
    return 1
  fi
  if ! grep -Eq 'failed to listen.*9090|address already in use' "$output"; then
    echo "pulsard start did not report the expected gRPC bind failure" >&2
    cat "$output" >&2
    return 1
  fi
}

case "$MODE" in
  shared | per-validator | external) ;;
  *)
    echo "usage: $0 <shared|per-validator|external>" >&2
    exit 2
    ;;
esac

require_cmd docker
require_cmd git
require_cmd grpcurl
require_cmd node
require_cmd npm
require_cmd python3
require_cmd timeout
if [[ "$ENABLE_VERIFIER_SIDECARS" != "0" && "$ENABLE_VERIFIER_SIDECARS" != "1" ]]; then
  echo "ENABLE_VERIFIER_SIDECARS must be 0 or 1" >&2
  exit 1
fi
docker compose version >/dev/null

if [[ "$(git -C "$WRAPPER_SOURCE" rev-parse HEAD)" != "$EXPECTED_WRAPPER_SHA" ]]; then
  echo "archive-wrapper checkout must be at $EXPECTED_WRAPPER_SHA" >&2
  exit 1
fi

docker buildx build --load --platform linux/amd64 -t "$PULSAR_IMAGE" "$REPO_ROOT"
docker buildx build --load --platform linux/amd64 -t "$WRAPPER_IMAGE" "$WRAPPER_SOURCE"
if [[ "$ENABLE_VERIFIER_SIDECARS" == "1" ]]; then
  docker buildx build --load --platform linux/amd64 -t "$VERIFIER_IMAGE" "$VERIFIER_SOURCE"
fi

MINA_PUBLIC_KEY="$(
  docker run --rm --entrypoint /usr/local/bin/pulsar-devtools "$PULSAR_IMAGE" \
    derive-mina-pub --address "$E2E_USER_MINA_PRIV_KEY"
)"
python3 "$SCRIPT_DIR/setup_local_testnet_helper.py" render-e2e-seed \
  --template "$SCRIPT_DIR/e2e/archive-wrapper-seed.sql.tmpl" \
  --output "$SEED_FILE" \
  --mina-public-key "$MINA_PUBLIC_KEY"
postgres_args=(render-postgres-compose \
  --output "$POSTGRES_COMPOSE_FILE" \
  --schema "$WRAPPER_SOURCE/fetchmina/sql/schema.sql" \
  --seed "$SEED_FILE")
if [[ "$MODE" == "external" ]]; then
  docker network create "$EXTERNAL_NETWORK" >/dev/null
  EXTERNAL_NETWORK_CREATED=1
  postgres_args+=(--network-key archive-wrapper-external)
fi
python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" "${postgres_args[@]}"

export ARCHIVE_WRAPPER_MODE="$MODE"
export ARCHIVE_WRAPPER_IMAGE="$WRAPPER_IMAGE"
export PULSAR_DOCKER_IMAGE="$PULSAR_IMAGE"
export POSTGRES_URI
export BRIDGE_CONFIRMATION_DEPTH=2
export BRIDGE_START_BLOCK_HEIGHT=10
export BRIDGE_MAX_BLOCK_RANGE=100
export E2E_USER_MINA_PRIV_KEY
export E2E_MIN_GAS_PRICE=0pmina
export PULSAR_DOCKER_PROJECT="$PROJECT"
export PULSAR_DOCKER_STATE_ROOT="$DOCKER_STATE_ROOT"
if [[ "$ENABLE_VERIFIER_SIDECARS" == "1" ]]; then
  export PULSAR_VERIFIER_IMAGE="$VERIFIER_IMAGE"
fi

if [[ "$MODE" == "external" ]]; then
  export ARCHIVE_WRAPPER_EXTERNAL_ADDRESS="external-wrapper:9095"
  export ARCHIVE_WRAPPER_EXTERNAL_TRANSPORT_MODE="trusted-network"
  export ARCHIVE_WRAPPER_EXTERNAL_NETWORK="$EXTERNAL_NETWORK"
fi

bash "$SCRIPT_DIR/docker_testnet.sh" config "$VALIDATOR_COUNT" >/dev/null
COMPOSE_STARTED=1
compose up -d --wait postgres
compose up --no-build --abort-on-container-failure --exit-code-from setup setup

if [[ "$MODE" == "external" ]]; then
  if compose run --rm --no-deps validator1 >/dev/null 2>&1; then
    echo "validator start unexpectedly succeeded before the external wrapper was ready" >&2
    exit 1
  fi

  EXTERNAL_CONFIG="$GENERATED_DIR/external-wrapper.yaml"
  mkdir -p "$GENERATED_DIR"
  python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" render-wrapper-config \
    --output "$EXTERNAL_CONFIG" \
    --network-id testnet
  docker volume create "$EXTERNAL_DATA_VOLUME" >/dev/null
  docker run -d \
    --name "$EXTERNAL_WRAPPER_NAME" \
    --network "$EXTERNAL_NETWORK" \
    --network-alias external-wrapper \
    --read-only \
    --restart unless-stopped \
    --env "POSTGRES_URI=$POSTGRES_URI" \
    --mount "type=bind,src=$EXTERNAL_CONFIG,dst=/etc/archive-wrapper/config.yaml,readonly" \
    --mount "type=volume,src=${PROJECT}_validator1_data,dst=/var/lib/pulsar,readonly" \
    --mount "type=volume,src=$EXTERNAL_DATA_VOLUME,dst=/var/lib/archive-wrapper" \
    --tmpfs /run/archive-wrapper:rw,uid=65532,gid=65532,mode=0700 \
    "$WRAPPER_IMAGE" >/dev/null
  EXTERNAL_WRAPPER_STARTED=1
  wait_for_container_health "$EXTERNAL_WRAPPER_NAME"
fi

if [[ "$MODE" == "shared" ]]; then
  compose up --no-build -d --wait --wait-timeout 120 archive-wrapper
  verify_grpc_bind_failure
fi

compose up --no-build -d --wait --wait-timeout 240 validator1 validator2 validator3
if [[ "$ENABLE_VERIFIER_SIDECARS" == "1" ]]; then
  compose up --no-build -d --wait --wait-timeout 240 verifier1 verifier2 verifier3
fi

case "$MODE" in
  shared) wrapper_endpoints=(archive-wrapper:9095) ;;
  per-validator)
    wrapper_endpoints=(
      archive-wrapper-validator1:9095
      archive-wrapper-validator2:9095
      archive-wrapper-validator3:9095
    )
    ;;
  external) wrapper_endpoints=(external-wrapper:9095) ;;
esac

for query_index in "${!wrapper_endpoints[@]}"; do
  compose exec -T validator1 pulsar-devtools query-archive-wrapper \
    --address "${wrapper_endpoints[query_index]}" \
    --transport-mode trusted-network \
    --latest 9 \
    --target 12 >"$TMP_DIR/wrapper-query-${query_index}.json"
  python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" assert-wrapper-query \
    --input "$TMP_DIR/wrapper-query-${query_index}.json"
done
if [[ "$MODE" == "per-validator" ]]; then
  cmp "$TMP_DIR/wrapper-query-0.json" "$TMP_DIR/wrapper-query-1.json"
  cmp "$TMP_DIR/wrapper-query-0.json" "$TMP_DIR/wrapper-query-2.json"
fi

VALIDATOR1_ADDRESS="$(
  compose exec -T validator1 pulsard keys show validator1 \
    --address --home /testnet/.pulsar-node1 --keyring-backend test
)"
compose exec -T validator1 pulsard query bank balances "$VALIDATOR1_ADDRESS" \
  --node tcp://127.0.0.1:26657 --output json >"$TMP_DIR/balance-before.json"
BALANCE_BEFORE="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" bank-balance --input "$TMP_DIR/balance-before.json" --denom pmina)"

compose exec -T validator1 pulsard tx bridge push-new-actions 12 \
  --from validator1 \
  --home /testnet/.pulsar-node1 \
  --keyring-backend test \
  --chain-id mytestnet \
  --node tcp://127.0.0.1:26657 \
  --gas auto \
  --gas-adjustment 1.5 \
  --fees 0pmina \
  --yes \
  --output json >"$TMP_DIR/tx-broadcast.json"
TX_HASH="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value --input "$TMP_DIR/tx-broadcast.json" --path txhash)"
wait_for_tx "$TX_HASH" "$TMP_DIR/tx-result.json"
TX_CODE="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value --input "$TMP_DIR/tx-result.json" --path code)"
if [[ "$TX_CODE" != "0" ]]; then
  echo "bridge transaction failed with code $TX_CODE" >&2
  exit 1
fi
TX_HEIGHT="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value --input "$TMP_DIR/tx-result.json" --path height)"

compose exec -T validator1 pulsard query bank balances "$VALIDATOR1_ADDRESS" \
  --node tcp://127.0.0.1:26657 --output json >"$TMP_DIR/balance-after.json"
BALANCE_AFTER="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" bank-balance --input "$TMP_DIR/balance-after.json" --denom pmina)"
if (( BALANCE_AFTER - BALANCE_BEFORE != 42 )); then
  echo "expected validator1 pmina balance to increase by 42, got $((BALANCE_AFTER - BALANCE_BEFORE))" >&2
  exit 1
fi

for index in 1 2 3; do
  compose exec -T "validator${index}" pulsard query bridge actions-reduced-root \
    --node tcp://127.0.0.1:26657 --output json >"$TMP_DIR/root-${index}.json"
  curl -fsS "http://127.0.0.1:${HOST_RPC_PORTS[index]}/block?height=$TX_HEIGHT" \
    >"$TMP_DIR/block-${index}.json"
done
cmp "$TMP_DIR/root-1.json" "$TMP_DIR/root-2.json"
cmp "$TMP_DIR/root-1.json" "$TMP_DIR/root-3.json"
APP_HASH_1="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value --input "$TMP_DIR/block-1.json" --path result.block.header.app_hash)"
APP_HASH_2="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value --input "$TMP_DIR/block-2.json" --path result.block.header.app_hash)"
APP_HASH_3="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value --input "$TMP_DIR/block-3.json" --path result.block.header.app_hash)"
[[ "$APP_HASH_1" == "$APP_HASH_2" && "$APP_HASH_1" == "$APP_HASH_3" ]]

(
  cd "$SCRIPT_DIR/vote-ext-verifier"
  npm ci >/dev/null
  node verify-vote-extensions.mjs \
    --grpc "127.0.0.1:${HOST_GRPC_PORTS[1]}" \
    --rpc "http://127.0.0.1:${HOST_RPC_PORTS[1]}" >/dev/null
)

case "$MODE" in
  shared)
    compose stop archive-wrapper
    for validator in validator1 validator2 validator3; do
      if validator_health "$validator" >/dev/null 2>&1; then
        echo "$validator remained healthy while the shared wrapper was stopped" >&2
        exit 1
      fi
    done
    compose start archive-wrapper
    compose up --no-build -d --wait --wait-timeout 120 archive-wrapper
    ;;
  per-validator)
    compose stop archive-wrapper-validator2
    validator_health validator1 >/dev/null
    validator_health validator3 >/dev/null
    if validator_health validator2 >/dev/null 2>&1; then
      echo "validator2 remained healthy while wrapper2 was stopped" >&2
      exit 1
    fi
    compose start archive-wrapper-validator2
    compose up --no-build -d --wait --wait-timeout 120 archive-wrapper-validator2

    LOCK_PROBE_NAME="${PROJECT}-lock-probe"
    set +e
    docker run --name "$LOCK_PROBE_NAME" \
      --network "${PROJECT}_default" \
      --read-only \
      --env "POSTGRES_URI=$POSTGRES_URI" \
      --mount "type=bind,src=$GENERATED_DIR/archive-wrapper-validator2.yaml,dst=/etc/archive-wrapper/config.yaml,readonly" \
      --mount "type=volume,src=${PROJECT}_validator2_data,dst=/var/lib/pulsar,readonly" \
      --mount "type=volume,src=${PROJECT}_archive-wrapper-validator2_data,dst=/var/lib/archive-wrapper" \
      --tmpfs /run/archive-wrapper:rw,uid=65532,gid=65532,mode=0700 \
      "$WRAPPER_IMAGE" >"$TMP_DIR/lock-probe.log" 2>&1
    lock_status=$?
    set -e
    docker rm "$LOCK_PROBE_NAME" >/dev/null 2>&1 || true
    if (( lock_status == 0 )) || ! grep -Fq "database is locked" "$TMP_DIR/lock-probe.log"; then
      echo "second wrapper did not fail with the expected LevelDB lock error" >&2
      exit 1
    fi
    ;;
  external)
    ;;
esac

for validator in validator1 validator2 validator3; do
  validator_health "$validator" >/dev/null
done

if [[ "$ENABLE_VERIFIER_SIDECARS" == "1" ]]; then
  run_verifier_e2e
  echo "$MODE archive-wrapper deployment with verifier sidecars E2E passed"
  exit 0
fi

# Verification is deliberately disabled in this chain-only test. Registering a
# proof still exercises deterministic on-chain lifecycle state, while the NoOp
# provider produces no commitment or vote. The proof must therefore finalize as
# INCONCLUSIVE at H+5 without affecting block production or app-hash agreement.
for index in 1 2 3; do
	compose exec -T "validator${index}" grep -A8 '^\[verification\]$' \
		"/testnet/.pulsar-node${index}/config/app.toml" | grep -q '^enabled = false$'
done
VERIFICATION_PROOF_HASH="abababababababababababababababababababababababababababababababab"
VERIFICATION_PUBLIC_INPUTS_HASH="cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd"
VERIFICATION_KEY_HASH="efefefefefefefefefefefefefefefefefefefefefefefefefefefefefefefef"
VERIFICATION_ID="ba0732b384544a5cfbec6e18a4ac4623bcb46647f24440bb627dd9f8c529fb24"
compose exec -T validator1 pulsard tx verification submit-proof \
	"$VERIFICATION_PROOF_HASH" mina-pickles "$VERIFICATION_PUBLIC_INPUTS_HASH" "$VERIFICATION_KEY_HASH" \
	--from validator1 \
	--home /testnet/.pulsar-node1 \
	--keyring-backend test \
	--chain-id mytestnet \
	--node tcp://127.0.0.1:26657 \
	--gas auto \
	--gas-adjustment 1.5 \
	--fees 0pmina \
	--yes \
	--output json >"$TMP_DIR/verification-tx-broadcast.json"
VERIFICATION_TX_HASH="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-tx-broadcast.json" --path txhash)"
wait_for_tx "$VERIFICATION_TX_HASH" "$TMP_DIR/verification-tx-result.json"
VERIFICATION_TX_CODE="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-tx-result.json" --path code)"
if [[ "$VERIFICATION_TX_CODE" != "0" ]]; then
	echo "verification proof transaction failed with code $VERIFICATION_TX_CODE" >&2
	exit 1
fi
VERIFICATION_PROOF_HEIGHT="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-tx-result.json" --path height)"
compose exec -T validator1 pulsard query verification proof "$VERIFICATION_PROOF_HEIGHT" 0 \
	--height "$VERIFICATION_PROOF_HEIGHT" \
	--node tcp://127.0.0.1:26657 --output json >"$TMP_DIR/verification-pending.json"
PENDING_HEIGHT="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-pending.json" --path proof_key.submission_height)"
PENDING_TYPE="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-pending.json" --path State.value.pending.proof_type)"
PENDING_ID="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-pending.json" --path State.value.pending.verification_id)"
PENDING_PROOF_HASH="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-pending.json" --path State.value.pending.proof_hash)"
PENDING_PUBLIC_INPUTS_HASH="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-pending.json" --path State.value.pending.public_inputs_hash)"
PENDING_KEY_HASH="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-pending.json" --path State.value.pending.verification_key_hash)"
[[ "$PENDING_HEIGHT" == "$VERIFICATION_PROOF_HEIGHT" ]]
[[ "$PENDING_TYPE" == "PROOF_TYPE_MINA_PICKLES" ]]
[[ "$PENDING_ID" == "ugcys4RUSlz77G4YpKxGI7y0ZkfyREC7Yn3Z+MUp+yQ=" ]]
[[ "$PENDING_PROOF_HASH" == "q6urq6urq6urq6urq6urq6urq6urq6urq6urq6urq6s=" ]]
[[ "$PENDING_PUBLIC_INPUTS_HASH" == "zc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc0=" ]]
[[ "$PENDING_KEY_HASH" == "7+/v7+/v7+/v7+/v7+/v7+/v7+/v7+/v7+/v7+/v7+8=" ]]
compose exec -T validator1 pulsard query verification proof-by-verification-id "$VERIFICATION_ID" \
	--height "$VERIFICATION_PROOF_HEIGHT" \
	--node tcp://127.0.0.1:26657 --output json >"$TMP_DIR/verification-pending-by-id.json"
PENDING_BY_ID_HEIGHT="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-pending-by-id.json" --path proof_key.submission_height)"
[[ "$PENDING_BY_ID_HEIGHT" == "$VERIFICATION_PROOF_HEIGHT" ]]

VERIFICATION_FINAL_HEIGHT="$((VERIFICATION_PROOF_HEIGHT + 5))"
wait_for_height "$VERIFICATION_FINAL_HEIGHT"
compose exec -T validator1 pulsard query verification final-proof-result "$VERIFICATION_PROOF_HEIGHT" 0 \
	--node tcp://127.0.0.1:26657 --output json >"$TMP_DIR/verification-final.json"
VERIFICATION_STATUS="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-final.json" --path final_result.status)"
VALID_POWER="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-final.json" --path final_result.valid_voting_power --default 0)"
INVALID_POWER="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-final.json" --path final_result.invalid_voting_power --default 0)"
TOTAL_POWER="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-final.json" --path final_result.total_voting_power)"
POWER_THRESHOLD="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-final.json" --path final_result.voting_power_threshold)"
STORED_FINAL_HEIGHT="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-final.json" --path final_result.finalized_height)"
[[ "$VERIFICATION_STATUS" == "PROOF_STATUS_INCONCLUSIVE" ]]
[[ "$VALID_POWER" == "0" && "$INVALID_POWER" == "0" ]]
[[ "$STORED_FINAL_HEIGHT" == "$VERIFICATION_FINAL_HEIGHT" ]]
(( TOTAL_POWER > 0 ))
(( POWER_THRESHOLD == TOTAL_POWER * 2 / 3 + 1 ))
compose exec -T validator1 pulsard query verification proof-by-verification-id "$VERIFICATION_ID" \
	--node tcp://127.0.0.1:26657 --output json >"$TMP_DIR/verification-final-by-id.json"
FINAL_BY_ID_STATUS="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value \
	--input "$TMP_DIR/verification-final-by-id.json" --path State.value.final_result.status)"
[[ "$FINAL_BY_ID_STATUS" == "PROOF_STATUS_INCONCLUSIVE" ]]

for index in 1 2 3; do
	curl -fsS "http://127.0.0.1:${HOST_RPC_PORTS[index]}/block?height=$VERIFICATION_FINAL_HEIGHT" \
		>"$TMP_DIR/block-verification-${index}.json"
done
VERIFICATION_APP_HASH_1="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value --input "$TMP_DIR/block-verification-1.json" --path result.block.header.app_hash)"
VERIFICATION_APP_HASH_2="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value --input "$TMP_DIR/block-verification-2.json" --path result.block.header.app_hash)"
VERIFICATION_APP_HASH_3="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" json-value --input "$TMP_DIR/block-verification-3.json" --path result.block.header.app_hash)"
[[ "$VERIFICATION_APP_HASH_1" == "$VERIFICATION_APP_HASH_2" && "$VERIFICATION_APP_HASH_1" == "$VERIFICATION_APP_HASH_3" ]]

declare -a latest_heights
for index in 1 2 3; do
  status_file="$TMP_DIR/status-${index}.json"
  curl -fsS "http://127.0.0.1:${HOST_RPC_PORTS[index]}/status" >"$status_file"
  latest_heights[index]="$(python3 "$SCRIPT_DIR/e2e/archive_wrapper_e2e.py" \
    json-value --input "$status_file" --path result.sync_info.latest_block_height)"
  # Verification is disabled in this chain-only deployment, so the NoOp provider
  # returns no terminal proof results. Consensus and mandatory Mina extensions
  # must continue normally, and no private commitment journal should be created.
  compose exec -T "validator${index}" test ! -e \
    "/testnet/.pulsar-node${index}/data/verification_commitment_state.json"
done

echo "${MODE} E2E evidence: tx_height=${TX_HEIGHT} app_hash=${APP_HASH_1} latest_heights=${latest_heights[1]},${latest_heights[2]},${latest_heights[3]} validators=healthy verification_sidecar=noop verification_proof_height=${VERIFICATION_PROOF_HEIGHT} verification_status=${VERIFICATION_STATUS} verification_total_power=${TOTAL_POWER} verification_threshold=${POWER_THRESHOLD} verification_app_hash=${VERIFICATION_APP_HASH_1} verification_local_state=absent"

echo "$MODE archive-wrapper deployment E2E passed"
