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
  local project="topology-${mode}"
  local compose_file="$TMP_DIR/$project/compose.json"
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
    "PULSAR_DOCKER_STATE_ROOT=$TMP_DIR" \
    "PULSAR_DOCKER_PROJECT=$project" \
    "${mode_env[@]}" \
    bash "$SCRIPT_DIR/docker_testnet.sh" config 3 >/dev/null

  if grep -Fq "test-secret" "$compose_file"; then
    echo "generated $mode Compose file contains the PostgreSQL secret" >&2
    exit 1
  fi

  POSTGRES_URI="$POSTGRES_URI_VALUE" \
    docker compose \
      --project-name "$project" \
      -f "$compose_file" \
      config --format json >"$TMP_DIR/${mode}-normalized.json"

  python3 -m json.tool "$TMP_DIR/${mode}-normalized.json" >/dev/null

  env -u POSTGRES_URI docker compose \
    --project-name "$project" \
    -f "$compose_file" \
    config --format json >/dev/null

  env -u POSTGRES_URI docker compose \
    --project-name "$project" \
    -f "$compose_file" \
    down --remove-orphans >/dev/null
}

render_and_validate shared
render_and_validate per-validator
render_and_validate external

echo "wrapper topology generation passed"
