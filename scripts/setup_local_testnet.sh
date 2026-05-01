#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
PYTHON_HELPER="$SCRIPT_DIR/setup_local_testnet_helper.py"
GO_HELPER="$SCRIPT_DIR/derive_mina_pub.go"

LEGACY_CHAIN_HOME="${CHAIN_HOME:-$HOME/.pulsar}"
NODE1_HOME="${NODE1_HOME:-$HOME/.pulsar-node1}"
NODE2_HOME="${NODE2_HOME:-$HOME/.pulsar-node2}"
CHAIN_ID="${CHAIN_ID:-mytestnet}"
NODE1_MONIKER="${NODE1_MONIKER:-node1}"
NODE2_MONIKER="${NODE2_MONIKER:-node2}"
NODE1_KEY_NAME="${NODE1_KEY_NAME:-validator1}"
NODE2_KEY_NAME="${NODE2_KEY_NAME:-validator2}"
KEYRING_BACKEND="${KEYRING_BACKEND:-test}"
DENOM="${DENOM:-pmina}"
STAKE_AMOUNT="${STAKE_AMOUNT:-1000000000}"
BOND_AMOUNT="${BOND_AMOUNT:-100000000}"
MIN_GAS_PRICE="${MIN_GAS_PRICE:-0.0001pmina}"
VOTE_EXT_ENABLE_HEIGHT="${VOTE_EXT_ENABLE_HEIGHT:-2}"
NODE1_MINA_PRIV_KEY="${NODE1_MINA_PRIV_KEY:-ES17xFroE2/QOa9yCLXsQ9sJMeIUVwr2ZXcdWGjNLlM=}"
NODE2_MINA_PRIV_KEY="${NODE2_MINA_PRIV_KEY:-PKeRXivUb4gZ/nMKxUK5beEnVJwIrzN71mAf7JVKsng=}"
NODE1_P2P_PORT="${NODE1_P2P_PORT:-26656}"
NODE1_RPC_PORT="${NODE1_RPC_PORT:-26657}"
NODE2_P2P_PORT="${NODE2_P2P_PORT:-26666}"
NODE2_RPC_PORT="${NODE2_RPC_PORT:-26667}"
NODE1_GRPC_PORT="${NODE1_GRPC_PORT:-9090}"
NODE2_GRPC_PORT="${NODE2_GRPC_PORT:-9091}"
NODE1_API_PORT="${NODE1_API_PORT:-1317}"
NODE2_API_PORT="${NODE2_API_PORT:-1318}"
BIN_DIR="${BIN_DIR:-$HOME/go/bin}"
BINARY_PATH="${BINARY_PATH:-$BIN_DIR/pulsard}"
COMPAT_BINARY_PATH="${COMPAT_BINARY_PATH:-$BIN_DIR/pulsar-chaind}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

derive_mina_pub_key() {
  go run "$GO_HELPER" "$1"
}

read_consensus_pub_key() {
  python3 "$PYTHON_HELPER" read-consensus-pub-key --priv-validator-key "$1"
}

configure_node() {
  local home="$1"
  local rpc_port="$2"
  local p2p_port="$3"
  local api_port="$4"
  local grpc_port="$5"
  local persistent_peers="$6"
  local mina_priv_key="$7"

  sed -i.bak "s|laddr = \"tcp://127.0.0.1:26657\"|laddr = \"tcp://0.0.0.0:${rpc_port}\"|" "$home/config/config.toml"
  sed -i.bak "s|laddr = \"tcp://0.0.0.0:26656\"|laddr = \"tcp://0.0.0.0:${p2p_port}\"|" "$home/config/config.toml"
  sed -i.bak "s|persistent_peers = \"\"|persistent_peers = \"${persistent_peers}\"|" "$home/config/config.toml"
  sed -i.bak 's|addr_book_strict = true|addr_book_strict = false|' "$home/config/config.toml"
  sed -i.bak 's|allow_duplicate_ip = false|allow_duplicate_ip = true|' "$home/config/config.toml"

  sed -i.bak "s|address = \"tcp://localhost:1317\"|address = \"tcp://localhost:${api_port}\"|" "$home/config/app.toml"
  sed -i.bak "s|address = \"localhost:9090\"|address = \"localhost:${grpc_port}\"|" "$home/config/app.toml"

  python3 "$PYTHON_HELPER" update-app-config \
    --app "$home/config/app.toml" \
    --min-gas-price "$MIN_GAS_PRICE" \
    --mina-priv-key "$mina_priv_key"
}

require_cmd go
require_cmd python3

NODE1_MINA_PUB_KEY="$(derive_mina_pub_key "$NODE1_MINA_PRIV_KEY")"
NODE2_MINA_PUB_KEY="$(derive_mina_pub_key "$NODE2_MINA_PRIV_KEY")"

NODE1_GENESIS_FILE="$NODE1_HOME/config/genesis.json"
NODE2_GENESIS_FILE="$NODE2_HOME/config/genesis.json"

echo "==> Cleaning previous homes..."
rm -rf "$LEGACY_CHAIN_HOME" "$NODE1_HOME" "$NODE2_HOME"

echo "==> Building binary..."
mkdir -p "$BIN_DIR"
(
  cd "$REPO_ROOT"
  go build -o "$BINARY_PATH" ./cmd/pulsard
)

if [[ "$COMPAT_BINARY_PATH" != "$BINARY_PATH" ]]; then
  cp "$BINARY_PATH" "$COMPAT_BINARY_PATH"
fi

echo "==> Initializing nodes..."
"$BINARY_PATH" init "$NODE1_MONIKER" --chain-id "$CHAIN_ID" --home "$NODE1_HOME" >/dev/null 2>&1
"$BINARY_PATH" init "$NODE2_MONIKER" --chain-id "$CHAIN_ID" --home "$NODE2_HOME" >/dev/null 2>&1

echo "==> Setting vote extension enable height..."
python3 "$PYTHON_HELPER" set-vote-extension-height \
  --genesis "$NODE1_GENESIS_FILE" \
  --height "$VOTE_EXT_ENABLE_HEIGHT"

echo "==> Creating validator keys..."
"$BINARY_PATH" keys add "$NODE1_KEY_NAME" --home "$NODE1_HOME" --keyring-backend "$KEYRING_BACKEND" >/dev/null 2>&1
"$BINARY_PATH" keys add "$NODE2_KEY_NAME" --home "$NODE2_HOME" --keyring-backend "$KEYRING_BACKEND" >/dev/null 2>&1

NODE1_ADDR="$("$BINARY_PATH" keys show "$NODE1_KEY_NAME" --address --home "$NODE1_HOME" --keyring-backend "$KEYRING_BACKEND")"
NODE2_ADDR="$("$BINARY_PATH" keys show "$NODE2_KEY_NAME" --address --home "$NODE2_HOME" --keyring-backend "$KEYRING_BACKEND")"

echo "==> Adding genesis accounts..."
"$BINARY_PATH" genesis add-genesis-account "$NODE1_ADDR" "${STAKE_AMOUNT}${DENOM}" --home "$NODE1_HOME" >/dev/null 2>&1
"$BINARY_PATH" genesis add-genesis-account "$NODE2_ADDR" "${STAKE_AMOUNT}${DENOM}" --home "$NODE1_HOME" >/dev/null 2>&1

echo "==> Creating gentx for ${NODE1_KEY_NAME}..."
"$BINARY_PATH" genesis gentx "$NODE1_KEY_NAME" "${BOND_AMOUNT}${DENOM}" \
  --chain-id "$CHAIN_ID" \
  --home "$NODE1_HOME" \
  --keyring-backend "$KEYRING_BACKEND" >/dev/null 2>&1

echo "==> Copying shared genesis to node2..."
cp "$NODE1_GENESIS_FILE" "$NODE2_GENESIS_FILE"

echo "==> Creating gentx for ${NODE2_KEY_NAME}..."
"$BINARY_PATH" genesis gentx "$NODE2_KEY_NAME" "${BOND_AMOUNT}${DENOM}" \
  --chain-id "$CHAIN_ID" \
  --home "$NODE2_HOME" \
  --keyring-backend "$KEYRING_BACKEND" >/dev/null 2>&1

echo "==> Collecting gentxs..."
cp "$NODE2_HOME"/config/gentx/*.json "$NODE1_HOME/config/gentx/"
"$BINARY_PATH" genesis collect-gentxs --home "$NODE1_HOME" >/dev/null 2>&1

NODE1_COSMOS_PUB_KEY="$(read_consensus_pub_key "$NODE1_HOME/config/priv_validator_key.json")"
NODE2_COSMOS_PUB_KEY="$(read_consensus_pub_key "$NODE2_HOME/config/priv_validator_key.json")"

echo "==> Patching keyregistry validator key pairs..."
python3 "$PYTHON_HELPER" patch-keyregistry \
  --genesis "$NODE1_GENESIS_FILE" \
  --cosmos-key "$NODE1_COSMOS_PUB_KEY" \
  --mina-pub-key "$NODE1_MINA_PUB_KEY" \
  --cosmos-key "$NODE2_COSMOS_PUB_KEY" \
  --mina-pub-key "$NODE2_MINA_PUB_KEY" >/dev/null

echo "==> Validating final genesis..."
"$BINARY_PATH" genesis validate-genesis --home "$NODE1_HOME" >/dev/null 2>&1

echo "==> Copying final genesis to node2..."
cp "$NODE1_GENESIS_FILE" "$NODE2_GENESIS_FILE"

NODE1_ID="$("$BINARY_PATH" tendermint show-node-id --home "$NODE1_HOME")"
NODE2_ID="$("$BINARY_PATH" tendermint show-node-id --home "$NODE2_HOME")"

echo "==> Configuring node1..."
configure_node \
  "$NODE1_HOME" \
  "$NODE1_RPC_PORT" \
  "$NODE1_P2P_PORT" \
  "$NODE1_API_PORT" \
  "$NODE1_GRPC_PORT" \
  "$NODE2_ID@127.0.0.1:$NODE2_P2P_PORT" \
  "$NODE1_MINA_PRIV_KEY"

echo "==> Configuring node2..."
configure_node \
  "$NODE2_HOME" \
  "$NODE2_RPC_PORT" \
  "$NODE2_P2P_PORT" \
  "$NODE2_API_PORT" \
  "$NODE2_GRPC_PORT" \
  "$NODE1_ID@127.0.0.1:$NODE1_P2P_PORT" \
  "$NODE2_MINA_PRIV_KEY"

echo ""
echo "Setup complete."
echo "  binary:            $BINARY_PATH"
echo "  compat binary:     $COMPAT_BINARY_PATH"
echo "  node1 home:        $NODE1_HOME"
echo "  node2 home:        $NODE2_HOME"
echo "  node1 address:     $NODE1_ADDR"
echo "  node2 address:     $NODE2_ADDR"
echo "  node1 cosmos key:  $NODE1_COSMOS_PUB_KEY"
echo "  node2 cosmos key:  $NODE2_COSMOS_PUB_KEY"
echo "  node1 mina key:    $NODE1_MINA_PUB_KEY"
echo "  node2 mina key:    $NODE2_MINA_PUB_KEY"
echo ""
echo "Start the nodes in separate terminals:"
echo "  $BINARY_PATH start --home $NODE1_HOME"
echo "  $BINARY_PATH start --home $NODE2_HOME"
echo ""
echo "Validation examples:"
echo "  curl -s http://localhost:$NODE1_RPC_PORT/validators | python3 -m json.tool | grep total"
echo "  $BINARY_PATH query votepersistence vote-ext-body-by-height 5 --home $NODE1_HOME"
