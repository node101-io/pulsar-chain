#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
RENDERER="$SCRIPT_DIR/render_docker_testnet.py"

usage() {
  echo "usage: $0 <up|down|reset|config> [validator-count]" >&2
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

resolve_path() {
  python3 -c 'import os, sys; print(os.path.realpath(sys.argv[1]))' "$1"
}

COMMAND="${1:-}"
if [[ -z "$COMMAND" ]]; then
  usage
  exit 1
fi
shift

case "$COMMAND" in
  up | down | reset | config) ;;
  *)
    usage
    exit 1
    ;;
esac

if (( $# > 1 )); then
  usage
  exit 1
fi

VALIDATOR_COUNT="${1:-${VALIDATOR_COUNT:-2}}"
if ! [[ "$VALIDATOR_COUNT" =~ ^[0-9]+$ ]] || (( VALIDATOR_COUNT < 1 )); then
  echo "validator count must be a positive integer, got: $VALIDATOR_COUNT" >&2
  exit 1
fi

PROJECT_NAME="${PULSAR_DOCKER_PROJECT:-pulsar-testnet-${VALIDATOR_COUNT}}"
if ! [[ "$PROJECT_NAME" =~ ^[a-zA-Z0-9][a-zA-Z0-9_.-]*$ ]]; then
  echo "invalid PULSAR_DOCKER_PROJECT: $PROJECT_NAME" >&2
  exit 1
fi

require_cmd python3

STATE_ROOT="$(resolve_path "${PULSAR_DOCKER_STATE_ROOT:-$REPO_ROOT/.docker}")"
if [[ "$STATE_ROOT" == "/" ]]; then
  echo "PULSAR_DOCKER_STATE_ROOT must not be the filesystem root" >&2
  exit 1
fi

GENERATED_ROOT_PATH="$STATE_ROOT/$PROJECT_NAME"
if [[ -L "$GENERATED_ROOT_PATH" ]]; then
  echo "generated project path must not be a symbolic link: $GENERATED_ROOT_PATH" >&2
  exit 1
fi

GENERATED_ROOT="$(resolve_path "$GENERATED_ROOT_PATH")"
case "$GENERATED_ROOT/" in
  "$STATE_ROOT/"*) ;;
  *)
    echo "generated project path escapes the configured state root: $GENERATED_ROOT" >&2
    exit 1
    ;;
esac

if [[ "$GENERATED_ROOT" == "$REPO_ROOT" || (-n "${HOME:-}" && "$GENERATED_ROOT" == "$(resolve_path "$HOME")") ]]; then
  echo "generated project path resolves to a protected directory: $GENERATED_ROOT" >&2
  exit 1
fi

COMPOSE_FILE="$GENERATED_ROOT/compose.json"
GENERATED_DIR="$GENERATED_ROOT/wrapper-configs"
PROJECT_MARKER="$GENERATED_ROOT/.pulsar-docker-project"
PROJECT_MARKER_CONTENT="pulsar-docker-project:$PROJECT_NAME"
VALIDATOR_STARTUP_TIMEOUT="${VALIDATOR_STARTUP_TIMEOUT:-120}"

if ! [[ "$VALIDATOR_STARTUP_TIMEOUT" =~ ^[0-9]+$ ]] || (( VALIDATOR_STARTUP_TIMEOUT < 1 )); then
  echo "validator startup timeout must be a positive integer, got: $VALIDATOR_STARTUP_TIMEOUT" >&2
  exit 1
fi

render_compose_file() {
  local mode="${ARCHIVE_WRAPPER_MODE-}"
  if [[ -z "$mode" ]]; then
    echo "ARCHIVE_WRAPPER_MODE is required: shared, per-validator, or external" >&2
    exit 1
  fi

  local network_id
  network_id="$(python3 "$SCRIPT_DIR/setup_local_testnet_helper.py" read-mina-network-id --config "${CHAIN_CONFIG_PATH:-$REPO_ROOT/config.yml}")"

  local args=(
    --repo-root "$REPO_ROOT"
    --output "$COMPOSE_FILE"
    --generated-dir "$GENERATED_DIR"
    --validator-count "$VALIDATOR_COUNT"
    --mode "$mode"
    --mina-network-id "$network_id"
  )

  case "$mode" in
    shared | per-validator)
      if [[ -z "${ARCHIVE_WRAPPER_IMAGE-}" ]]; then
        echo "ARCHIVE_WRAPPER_IMAGE is required for $mode mode" >&2
        exit 1
      fi
      if [[ -z "${POSTGRES_URI-}" ]]; then
        echo "POSTGRES_URI is required for $mode mode" >&2
        exit 1
      fi
      args+=(--wrapper-image "$ARCHIVE_WRAPPER_IMAGE")
      ;;
    external)
      if [[ -z "${ARCHIVE_WRAPPER_EXTERNAL_ADDRESS-}" ]]; then
        echo "ARCHIVE_WRAPPER_EXTERNAL_ADDRESS is required for external mode" >&2
        exit 1
      fi
      if [[ -z "${ARCHIVE_WRAPPER_EXTERNAL_TRANSPORT_MODE-}" ]]; then
        echo "ARCHIVE_WRAPPER_EXTERNAL_TRANSPORT_MODE is required for external mode" >&2
        exit 1
      fi
      args+=(
        --external-address "$ARCHIVE_WRAPPER_EXTERNAL_ADDRESS"
        --external-transport-mode "$ARCHIVE_WRAPPER_EXTERNAL_TRANSPORT_MODE"
      )
      if [[ -n "${ARCHIVE_WRAPPER_EXTERNAL_NETWORK-}" ]]; then
        args+=(--external-network "$ARCHIVE_WRAPPER_EXTERNAL_NETWORK")
      fi
      ;;
    *)
      echo "invalid ARCHIVE_WRAPPER_MODE: $mode" >&2
      exit 1
      ;;
  esac

  if [[ "${ARCHIVE_WRAPPER_ADD_HOST_GATEWAY:-0}" == "1" ]]; then
    args+=(--add-host-gateway)
  fi
  if [[ -n "${PULSAR_VERIFIER_IMAGE:-}" ]]; then
    args+=(--verifier-image "$PULSAR_VERIFIER_IMAGE")
  fi

  python3 "$RENDERER" "${args[@]}"

  local marker_tmp="$PROJECT_MARKER.tmp.$$"
  (
    umask 077
    printf '%s\n' "$PROJECT_MARKER_CONTENT" >"$marker_tmp"
  )
  mv -f -- "$marker_tmp" "$PROJECT_MARKER"
}

validate_project_marker() {
  if [[ ! -f "$PROJECT_MARKER" || -L "$PROJECT_MARKER" ]]; then
    echo "generated project ownership marker is missing or invalid: $PROJECT_MARKER" >&2
    return 1
  fi

  local marker_content
  marker_content="$(cat -- "$PROJECT_MARKER")"
  if [[ "$marker_content" != "$PROJECT_MARKER_CONTENT" ]]; then
    echo "generated project ownership marker does not match project $PROJECT_NAME" >&2
    return 1
  fi
}

if [[ "$COMMAND" == "up" || "$COMMAND" == "config" ]]; then
  require_cmd python3
  render_compose_file
else
  validate_project_marker
  if [[ ! -f "$COMPOSE_FILE" ]]; then
    echo "generated Compose file does not exist: $COMPOSE_FILE" >&2
    echo "run '$0 config $VALIDATOR_COUNT' with an explicit ARCHIVE_WRAPPER_MODE first" >&2
    exit 1
  fi
fi

if [[ "$COMMAND" == "config" ]]; then
  cat "$COMPOSE_FILE"
  exit 0
fi

require_cmd docker

run_compose() {
  docker compose --project-name "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

declare -a START_SERVICES=()
for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  START_SERVICES+=("validator${i}")
  if [[ -n "${PULSAR_VERIFIER_IMAGE:-}" ]]; then
    START_SERVICES+=("verifier${i}")
  fi
done

cleanup_partial_environment() {
  echo "startup failed; partial compose state before cleanup:" >&2
  run_compose ps -a >&2 || true
  echo "cleaning up containers and network; preserving named volumes." >&2
  run_compose down --remove-orphans >/dev/null 2>&1 || true
  echo "named volumes were preserved; run '$0 reset ${VALIDATOR_COUNT}' to remove them." >&2
}

case "$COMMAND" in
  up)
    run_compose build setup
    if ! run_compose up --no-build --abort-on-container-failure --exit-code-from setup setup; then
      cleanup_partial_environment
      exit 1
    fi
    if ! run_compose up --no-build -d --wait \
      --wait-timeout "$VALIDATOR_STARTUP_TIMEOUT" \
      "${START_SERVICES[@]}"; then
      cleanup_partial_environment
      exit 1
    fi
    run_compose ps
    ;;
  down)
    run_compose down --remove-orphans
    ;;
  reset)
    run_compose down --volumes --remove-orphans
    rm -rf -- "$GENERATED_ROOT"
    ;;
  *)
    usage
    exit 1
    ;;
esac
