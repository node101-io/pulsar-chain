#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
TMP_DIR="$(mktemp -d)"
POSTGRES_URI_VALUE="postgres://archive:test-secret@postgres:5432/archive?sslmode=disable"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

require_cmd docker
require_cmd python3
docker compose version >/dev/null

render_and_validate() {
  local mode="$1"
  local compose_file="$TMP_DIR/${mode}.json"
  local generated_dir="$TMP_DIR/${mode}-generated"
  local -a mode_env

  case "$mode" in
    shared | per-validator)
      mode_env=(
        "ARCHIVE_WRAPPER_IMAGE=archive-wrapper:test"
        "POSTGRES_URI=$POSTGRES_URI_VALUE"
      )
      ;;
    external)
      mode_env=(
        "ARCHIVE_WRAPPER_EXTERNAL_ADDRESS=external-wrapper:9095"
        "ARCHIVE_WRAPPER_EXTERNAL_TRANSPORT_MODE=trusted-network"
        "ARCHIVE_WRAPPER_EXTERNAL_NETWORK=wrapper-external"
      )
      ;;
  esac

  env \
    "ARCHIVE_WRAPPER_MODE=$mode" \
    "COMPOSE_FILE=$compose_file" \
    "GENERATED_DIR=$generated_dir" \
    "PULSAR_DOCKER_PROJECT=topology-${mode}" \
    "${mode_env[@]}" \
    bash "$SCRIPT_DIR/docker_testnet.sh" config 3 >/dev/null

  if grep -Fq "test-secret" "$compose_file"; then
    echo "generated $mode Compose file contains the PostgreSQL secret" >&2
    exit 1
  fi

  POSTGRES_URI="$POSTGRES_URI_VALUE" \
    docker compose \
      --project-name "topology-${mode}" \
      -f "$compose_file" \
      config --format json >"$TMP_DIR/${mode}-normalized.json"

  python3 -m json.tool "$TMP_DIR/${mode}-normalized.json" >/dev/null
}

render_and_validate shared
render_and_validate per-validator
render_and_validate external

echo "wrapper topology generation passed"
