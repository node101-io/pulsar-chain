#!/usr/bin/env bash

set -euo pipefail

TESTNET_ROOT="${DOCKER_TESTNET_ROOT:-/testnet}"

if (( $# == 0 )); then
  echo "explicit command required" >&2
  echo "examples:" >&2
  echo "  setup-local-testnet 3" >&2
  echo "  start-validator 1" >&2
  exit 1
fi

if [[ "${1:-}" == "setup-local-testnet" ]]; then
  shift

  mkdir -p /tmp/go-build "$TESTNET_ROOT"

  export HOME="$TESTNET_ROOT"
  export BINARY_PATH="/usr/local/bin/pulsard"
  export COMPAT_BINARY_PATH="/usr/local/bin/pulsard"
  export BIN_DIR="/tmp/pulsar-bin"
  export GOCACHE="/tmp/go-build"
  export API_BIND_HOST="0.0.0.0"
  export SKIP_BUILD="1"
  export SETUP_CONTEXT="container"
  export START_VALIDATORS="0"

  exec /app/scripts/setup_local_testnet.sh "$@"
fi

if [[ "${1:-}" == "start-validator" ]]; then
  shift

  VALIDATOR_INDEX="${1:-}"
  if ! [[ "$VALIDATOR_INDEX" =~ ^[0-9]+$ ]] || (( VALIDATOR_INDEX < 1 )); then
    echo "usage: start-validator <validator-index>" >&2
    exit 1
  fi

  VALIDATOR_HOME="${TESTNET_ROOT}/.pulsar-node${VALIDATOR_INDEX}"
  if [[ ! -d "$VALIDATOR_HOME" ]]; then
    echo "validator home does not exist: $VALIDATOR_HOME" >&2
    echo "run setup-local-testnet first against the mounted Docker volume" >&2
    exit 1
  fi

  exec /usr/local/bin/pulsard start --home "$VALIDATOR_HOME"
fi

exec /usr/local/bin/pulsard "$@"
