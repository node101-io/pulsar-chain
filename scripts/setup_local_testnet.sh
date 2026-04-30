#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
PYTHON_HELPER="$SCRIPT_DIR/setup_local_testnet_helper.py"
GO_HELPER="$SCRIPT_DIR/derive_mina_pub.go"

CHAIN_HOME="${CHAIN_HOME:-$HOME/.pulsar-chain}"
CHAIN_ID="${CHAIN_ID:-mytestnet}"
MONIKER="${MONIKER:-mynode}"
KEY_NAME="${KEY_NAME:-alice}"
KEYRING_BACKEND="${KEYRING_BACKEND:-test}"
DENOM="${DENOM:-pmina}"
GENESIS_BALANCE="${GENESIS_BALANCE:-1000000000}"
BOND_AMOUNT="${BOND_AMOUNT:-100000000}"
MIN_GAS_PRICE="${MIN_GAS_PRICE:-0.0001pmina}"
VOTE_EXT_ENABLE_HEIGHT="${VOTE_EXT_ENABLE_HEIGHT:-1}"
CONFIG_YML_PATH="${CONFIG_YML_PATH:-$REPO_ROOT/config.yml}"
TMP_BUILD_DIR="$(mktemp -d "${TMPDIR:-/tmp}/pulsar-localnet.XXXXXX")"
BINARY_PATH="$TMP_BUILD_DIR/pulsar-chaind"

cleanup() {
  rm -rf "$TMP_BUILD_DIR"
}

trap cleanup EXIT

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

get_mina_priv_key() {
  if [[ -n "${MINA_PRIV_KEY:-}" ]]; then
    printf '%s\n' "$MINA_PRIV_KEY"
    return
  fi

  python3 "$PYTHON_HELPER" read-mina-priv-key --config "$CONFIG_YML_PATH"
}

derive_mina_pub_key() {
  go run "$GO_HELPER" "$1"
}

require_cmd go
require_cmd python3

MINA_PRIV_KEY="$(get_mina_priv_key)"
MINA_PUB_KEY="$(derive_mina_pub_key "$MINA_PRIV_KEY")"

GENESIS_FILE="$CHAIN_HOME/config/genesis.json"
APP_FILE="$CHAIN_HOME/config/app.toml"

echo "==> Cleaning previous homes..."
rm -rf "$CHAIN_HOME" "$HOME/.pulsar-node1" "$HOME/.pulsar-node2"

echo "==> Building temporary binary..."
(
  cd "$REPO_ROOT"
  go build -o "$BINARY_PATH" ./cmd/pulsard
)

echo "==> Initializing chain home..."
"$BINARY_PATH" init "$MONIKER" --chain-id "$CHAIN_ID" --home "$CHAIN_HOME" >/dev/null

echo "==> Setting vote extension enable height..."
python3 "$PYTHON_HELPER" set-vote-extension-height \
  --genesis "$GENESIS_FILE" \
  --height "$VOTE_EXT_ENABLE_HEIGHT"

echo "==> Creating validator key..."
"$BINARY_PATH" keys add "$KEY_NAME" --home "$CHAIN_HOME" --keyring-backend "$KEYRING_BACKEND" >/dev/null

VALIDATOR_ADDR="$("$BINARY_PATH" keys show "$KEY_NAME" --address --home "$CHAIN_HOME" --keyring-backend "$KEYRING_BACKEND")"

echo "==> Adding genesis account for $KEY_NAME..."
"$BINARY_PATH" genesis add-genesis-account "$VALIDATOR_ADDR" "${GENESIS_BALANCE}${DENOM}" --home "$CHAIN_HOME" >/dev/null

echo "==> Creating gentx..."
"$BINARY_PATH" genesis gentx "$KEY_NAME" "${BOND_AMOUNT}${DENOM}" \
  --chain-id "$CHAIN_ID" \
  --home "$CHAIN_HOME" \
  --keyring-backend "$KEYRING_BACKEND" >/dev/null

echo "==> Collecting gentxs..."
"$BINARY_PATH" genesis collect-gentxs --home "$CHAIN_HOME" >/dev/null

echo "==> Patching keyregistry validator key pair from genesis validator pubkey..."
COSMOS_PUB_KEY="$(python3 "$PYTHON_HELPER" patch-keyregistry \
  --genesis "$GENESIS_FILE" \
  --mina-pub-key "$MINA_PUB_KEY")"

echo "==> Writing vote extension private key to app.toml..."
python3 "$PYTHON_HELPER" update-app-config \
  --app "$APP_FILE" \
  --min-gas-price "$MIN_GAS_PRICE" \
  --mina-priv-key "$MINA_PRIV_KEY"

echo "==> Validating final genesis..."
"$BINARY_PATH" genesis validate-genesis --home "$CHAIN_HOME" >/dev/null

echo ""
echo "Setup complete."
echo "  home:        $CHAIN_HOME"
echo "  validator:   $KEY_NAME"
echo "  cosmos_key:  $COSMOS_PUB_KEY"
echo "  mina_key:    $MINA_PUB_KEY"
echo ""
echo "==> Starting chain..."
"$BINARY_PATH" start --home "$CHAIN_HOME"
