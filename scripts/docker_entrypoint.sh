#!/usr/bin/env bash

set -euo pipefail

if [[ "${1:-}" == "setup-local-testnet" ]]; then
  shift

  HOST_HOME="${HOST_HOME:-}"
  if [[ -z "$HOST_HOME" ]]; then
    echo "HOST_HOME must point to your mounted host home directory." >&2
    echo 'example: docker run --rm -e HOST_HOME="$HOME" -v "$HOME:$HOME" pulsar-local-testnet setup-local-testnet 3' >&2
    exit 1
  fi

  if [[ ! -d "$HOST_HOME" ]]; then
    echo "HOST_HOME does not exist inside the container: $HOST_HOME" >&2
    exit 1
  fi

  mkdir -p /tmp/go-build

  export HOME="$HOST_HOME"
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

exec /usr/local/bin/pulsard "$@"
