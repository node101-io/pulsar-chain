#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
WRAPPER_SOURCE="${ARCHIVE_WRAPPER_SOURCE:-$(cd -- "$REPO_ROOT/../archive-wrapper" && pwd)}"
EXPECTED_WRAPPER_SHA="6141249715c539cb3f16142d12e494af10684baf"
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
    "$TMP_DIR"/root-*.json "$TMP_DIR"/block-*.json; do
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
docker compose version >/dev/null

if [[ "$(git -C "$WRAPPER_SOURCE" rev-parse HEAD)" != "$EXPECTED_WRAPPER_SHA" ]]; then
  echo "archive-wrapper checkout must be at $EXPECTED_WRAPPER_SHA" >&2
  exit 1
fi

docker buildx build --load --platform linux/amd64 -t "$PULSAR_IMAGE" "$REPO_ROOT"
docker buildx build --load --platform linux/amd64 -t "$WRAPPER_IMAGE" "$WRAPPER_SOURCE"

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

compose up --no-build -d --wait --wait-timeout 240 validator1 validator2 validator3

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

echo "$MODE archive-wrapper deployment E2E passed"
