#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"

usage() {
  echo "usage: $0 <up|down|reset|config> [validator-count]" >&2
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

host_p2p_port() {
  local index="$1"
  printf '%s\n' "$((26656 + ((index - 1) * 10)))"
}

host_rpc_port() {
  local index="$1"
  printf '%s\n' "$((26657 + ((index - 1) * 10)))"
}

host_api_port() {
  local index="$1"
  printf '%s\n' "$((1317 + index - 1))"
}

host_grpc_port() {
  local index="$1"
  printf '%s\n' "$((9090 + index - 1))"
}

host_pprof_port() {
  local index="$1"
  printf '%s\n' "$((6060 + index - 1))"
}

write_compose_file() {
  local compose_file="$1"
  local validator_count="$2"
  local i

  mkdir -p "$(dirname "$compose_file")"

  cat > "$compose_file" <<EOF
x-pulsar-image: &pulsar-image
  image: \${PULSAR_DOCKER_IMAGE:-pulsar-chain:local}

x-pulsar-build: &pulsar-build
  build:
    context: ..
    dockerfile: Dockerfile

services:
  setup:
    <<: [*pulsar-image, *pulsar-build]
    command: ["setup-local-testnet", "${validator_count}"]
    restart: "no"
    volumes:
EOF

  for ((i = 1; i <= validator_count; i++)); do
    cat >> "$compose_file" <<EOF
      - validator${i}_data:/testnet/.pulsar-node${i}
EOF
  done

  for ((i = 1; i <= validator_count; i++)); do
    cat >> "$compose_file" <<EOF

  validator${i}:
    <<: *pulsar-image
    command: ["start-validator", "${i}"]
    depends_on:
      setup:
        condition: service_completed_successfully
    restart: unless-stopped
    volumes:
      - validator${i}_data:/testnet/.pulsar-node${i}
    ports:
      - "$(host_p2p_port "$i"):26656"
      - "$(host_rpc_port "$i"):26657"
      - "$(host_api_port "$i"):1317"
      - "$(host_grpc_port "$i"):9090"
      - "$(host_pprof_port "$i"):6060"
    healthcheck:
      test: ["CMD", "/opt/pulsar/scripts/docker_entrypoint.sh", "healthcheck-validator"]
      interval: 5s
      timeout: 3s
      retries: 20
      start_period: 20s
EOF
  done

  cat >> "$compose_file" <<EOF

volumes:
EOF

  for ((i = 1; i <= validator_count; i++)); do
    cat >> "$compose_file" <<EOF
  validator${i}_data:
EOF
  done
}

COMMAND="${1:-}"
if [[ -z "$COMMAND" ]]; then
  usage
  exit 1
fi
shift || true

VALIDATOR_COUNT="${1:-${VALIDATOR_COUNT:-2}}"
if ! [[ "$VALIDATOR_COUNT" =~ ^[0-9]+$ ]] || (( VALIDATOR_COUNT < 1 )); then
  echo "validator count must be a positive integer, got: $VALIDATOR_COUNT" >&2
  exit 1
fi

COMPOSE_FILE="${COMPOSE_FILE:-$REPO_ROOT/.docker/docker-compose.testnet.${VALIDATOR_COUNT}.yml}"
PROJECT_NAME="${PULSAR_DOCKER_PROJECT:-pulsar-testnet-${VALIDATOR_COUNT}}"

write_compose_file "$COMPOSE_FILE" "$VALIDATOR_COUNT"

if [[ "$COMMAND" == "config" ]]; then
  cat "$COMPOSE_FILE"
  exit 0
fi

require_cmd docker

run_compose() {
  docker compose --project-name "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

case "$COMMAND" in
  up)
    run_compose build setup
    run_compose up --no-build -d
    run_compose ps
    ;;
  down)
    run_compose down --remove-orphans
    ;;
  reset)
    run_compose down --volumes --remove-orphans
    ;;
  *)
    usage
    exit 1
    ;;
esac
