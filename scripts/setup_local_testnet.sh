#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
PYTHON_HELPER="$SCRIPT_DIR/setup_local_testnet_helper.py"
DEVTOOLS_HELPER="$SCRIPT_DIR/devtools"
GOFLAGS_WITH_PUREGO="${GOFLAGS:-} -tags=purego"
CHAIN_CONFIG_PATH="${CHAIN_CONFIG_PATH:-$REPO_ROOT/config.yml}"

LEGACY_CHAIN_HOME="${CHAIN_HOME:-$HOME/.pulsar}"
CHAIN_ID="${CHAIN_ID:-mytestnet}"
KEYRING_BACKEND="${KEYRING_BACKEND:-test}"
DENOM="${DENOM:-pmina}"
STAKE_AMOUNT="${STAKE_AMOUNT:-1000000000}"
BOND_AMOUNT="${BOND_AMOUNT:-100000000}"
MIN_GAS_PRICE="${MIN_GAS_PRICE:-0.0001pmina}"
VOTE_EXT_ENABLE_HEIGHT="${VOTE_EXT_ENABLE_HEIGHT:-1}"
BIN_DIR="${BIN_DIR:-$HOME/go/bin}"
BINARY_PATH="${BINARY_PATH:-$BIN_DIR/pulsard}"
COMPAT_BINARY_PATH="${COMPAT_BINARY_PATH:-$BIN_DIR/pulsar-chaind}"
DEFAULT_NODE1_MINA_PRIV_KEY="ES17xFroE2/QOa9yCLXsQ9sJMeIUVwr2ZXcdWGjNLlM="
DEFAULT_NODE2_MINA_PRIV_KEY="PKeRXivUb4gZ/nMKxUK5beEnVJwIrzN71mAf7JVKsng="

if (( $# > 1 )); then
  echo "usage: $0 [validator-count]" >&2
  exit 1
fi

VALIDATOR_COUNT="${1:-${VALIDATOR_COUNT:-2}}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

derive_mina_pub_key() {
  GOFLAGS="$GOFLAGS_WITH_PUREGO" go run "$DEVTOOLS_HELPER" derive-mina-pub "$1"
}

read_consensus_pub_key() {
  python3 "$PYTHON_HELPER" read-consensus-pub-key --priv-validator-key "$1"
}

read_wrapper_grpc_address() {
  python3 "$PYTHON_HELPER" read-wrapper-grpc-address --config "$1"
}

read_bridge_genesis_param() {
  python3 "$PYTHON_HELPER" read-bridge-genesis-param --config "$1" --key "$2"
}

generate_default_mina_priv_key() {
  python3 "$PYTHON_HELPER" generate-default-mina-priv-key --index "$1"
}

default_node_mina_priv_key() {
  local index="$1"

  case "$index" in
    1) printf '%s\n' "$DEFAULT_NODE1_MINA_PRIV_KEY" ;;
    2) printf '%s\n' "$DEFAULT_NODE2_MINA_PRIV_KEY" ;;
    *) generate_default_mina_priv_key "$index" ;;
  esac
}

validate_mina_priv_key() {
  python3 "$PYTHON_HELPER" validate-mina-priv-key --index "$1" --mina-priv-key "$2"
}

read_mina_network_id() {
  python3 "$PYTHON_HELPER" read-mina-network-id --config "$1"
}

resolve_default_mina_network_id() {
  if [[ -n "${MINA_NETWORK_ID:-}" ]]; then
    printf '%s\n' "$MINA_NETWORK_ID"
    return
  fi

  if [[ -f "$CHAIN_CONFIG_PATH" ]]; then
    read_mina_network_id "$CHAIN_CONFIG_PATH"
    return
  fi

  printf '%s\n' "devnet"
}

resolve_default_bridge_param() {
  local env_var_name="$1"
  local config_key="$2"
  local fallback="$3"

  if [[ -n "${!env_var_name:-}" ]]; then
    printf '%s\n' "${!env_var_name}"
    return
  fi

  if [[ -f "$CHAIN_CONFIG_PATH" ]]; then
    local config_value
    config_value="$(read_bridge_genesis_param "$CHAIN_CONFIG_PATH" "$config_key")"
    if [[ -n "$config_value" ]]; then
      printf '%s\n' "$config_value"
      return
    fi
  fi

  printf '%s\n' "$fallback"
}

validate_mina_network_id() {
  local index="$1"
  local mina_network_id="$2"

  if [[ -z "$mina_network_id" ]]; then
    echo "node${index} mina network id must not be empty" >&2
    exit 1
  fi
}

validate_positive_int() {
  local value_name="$1"
  local value="$2"

  if ! [[ "$value" =~ ^[0-9]+$ ]] || (( value < 1 )); then
    echo "$value_name must be a positive integer, got: $value" >&2
    exit 1
  fi
}

validate_non_empty() {
  local value_name="$1"
  local contract_address="$2"

  if [[ -z "$contract_address" ]]; then
    echo "$value_name must not be empty" >&2
    exit 1
  fi
}

get_node_setting() {
  local index="$1"
  local suffix="$2"
  local default_value="$3"
  local var_name="NODE${index}_${suffix}"

  printf '%s\n' "${!var_name:-$default_value}"
}

configure_node() {
  local home="$1"
  local rpc_port="$2"
  local p2p_port="$3"
  local api_port="$4"
  local grpc_port="$5"
  local persistent_peers="$6"
  local mina_priv_key="$7"
  local mina_network_id="$8"
  local wrapper_grpc_address="$9"

  sed -i.bak "s|laddr = \"tcp://127.0.0.1:26657\"|laddr = \"tcp://0.0.0.0:${rpc_port}\"|" "$home/config/config.toml"
  sed -i.bak "s|laddr = \"tcp://0.0.0.0:26656\"|laddr = \"tcp://0.0.0.0:${p2p_port}\"|" "$home/config/config.toml"
  sed -i.bak "s|persistent_peers = \"\"|persistent_peers = \"${persistent_peers}\"|" "$home/config/config.toml"
  sed -i.bak 's|addr_book_strict = true|addr_book_strict = false|' "$home/config/config.toml"
  sed -i.bak 's|allow_duplicate_ip = false|allow_duplicate_ip = true|' "$home/config/config.toml"

  sed -i.bak "s|address = \"tcp://localhost:1317\"|address = \"tcp://localhost:${api_port}\"|" "$home/config/app.toml"
  sed -i.bak "s|address = \"localhost:9090\"|address = \"0.0.0.0:${grpc_port}\"|" "$home/config/app.toml"

  python3 "$PYTHON_HELPER" update-app-config \
    --app "$home/config/app.toml" \
    --min-gas-price "$MIN_GAS_PRICE" \
    --mina-priv-key "$mina_priv_key" \
    --mina-network-id "$mina_network_id" \
    --wrapper-grpc-address "$wrapper_grpc_address"
}

build_persistent_peers() {
  local current_index="$1"
  local i
  local -a peers=()

  for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
    if (( i == current_index )); then
      continue
    fi

    peers+=("${NODE_IDS[i]}@127.0.0.1:${NODE_P2P_PORTS[i]}")
  done

  local IFS=,
  printf '%s\n' "${peers[*]}"
}

require_cmd go
require_cmd python3

if ! [[ "$VALIDATOR_COUNT" =~ ^[0-9]+$ ]] || (( VALIDATOR_COUNT < 1 )); then
  echo "validator count must be a positive integer, got: $VALIDATOR_COUNT" >&2
  exit 1
fi

declare -a NODE_HOMES NODE_MONIKERS NODE_KEY_NAMES NODE_MINA_PRIV_KEYS NODE_MINA_PUB_KEYS
declare -a NODE_MINA_NETWORK_IDS
declare -a NODE_P2P_PORTS NODE_RPC_PORTS NODE_GRPC_PORTS NODE_API_PORTS
declare -a NODE_GENESIS_FILES NODE_ADDRS NODE_COSMOS_PUB_KEYS NODE_IDS
declare -a NODE_WRAPPER_GRPC_ADDRESSES

DEFAULT_WRAPPER_GRPC_ADDRESS="${WRAPPER_GRPC_ADDRESS:-}"
if [[ -z "$DEFAULT_WRAPPER_GRPC_ADDRESS" && -f "$CHAIN_CONFIG_PATH" ]]; then
  DEFAULT_WRAPPER_GRPC_ADDRESS="$(read_wrapper_grpc_address "$CHAIN_CONFIG_PATH")"
fi
if [[ -z "$DEFAULT_WRAPPER_GRPC_ADDRESS" ]]; then
  DEFAULT_WRAPPER_GRPC_ADDRESS="127.0.0.1:9095"
fi

DEFAULT_MINA_NETWORK_ID="$(resolve_default_mina_network_id)"
BRIDGE_CONFIRMATION_DEPTH="$(resolve_default_bridge_param "CONFIRMATION_DEPTH" "confirmation_depth" "32")"
BRIDGE_CONTRACT_ADDRESS="$(resolve_default_bridge_param "CONTRACT_ADDRESS" "contract_address" "B62qjRDirGFRf5dvNcGzMs5oWzQ2VyNcygnoKM2MkxB9PFUp7Utdraf")"
BRIDGE_START_BLOCK_HEIGHT="$(resolve_default_bridge_param "START_BLOCK_HEIGHT" "start_block_height" "1")"
BRIDGE_MAX_BLOCK_RANGE="$(resolve_default_bridge_param "MAX_BLOCK_RANGE" "max_block_range" "1000")"
BRIDGE_ACTIONS_REDUCED_ROOT_SNAPSHOT_WINDOW_SIZE="$(resolve_default_bridge_param "ACTIONS_REDUCED_ROOT_SNAPSHOT_WINDOW_SIZE" "actions_reduced_root_snapshot_window_size" "4")"

validate_positive_int "confirmation depth" "$BRIDGE_CONFIRMATION_DEPTH"
validate_non_empty "contract address" "$BRIDGE_CONTRACT_ADDRESS"
validate_positive_int "start block height" "$BRIDGE_START_BLOCK_HEIGHT"
validate_positive_int "max block range" "$BRIDGE_MAX_BLOCK_RANGE"
validate_positive_int "actions reduced root snapshot window size" "$BRIDGE_ACTIONS_REDUCED_ROOT_SNAPSHOT_WINDOW_SIZE"

for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  NODE_HOMES[i]="$(get_node_setting "$i" "HOME" "$HOME/.pulsar-node${i}")"
  NODE_MONIKERS[i]="$(get_node_setting "$i" "MONIKER" "node${i}")"
  NODE_KEY_NAMES[i]="$(get_node_setting "$i" "KEY_NAME" "validator${i}")"
  NODE_MINA_PRIV_KEYS[i]="$(get_node_setting "$i" "MINA_PRIV_KEY" "$(default_node_mina_priv_key "$i")")"
  validate_mina_priv_key "$i" "${NODE_MINA_PRIV_KEYS[i]}"
  NODE_MINA_NETWORK_IDS[i]="$(get_node_setting "$i" "MINA_NETWORK_ID" "$DEFAULT_MINA_NETWORK_ID")"
  validate_mina_network_id "$i" "${NODE_MINA_NETWORK_IDS[i]}"
  NODE_P2P_PORTS[i]="$(get_node_setting "$i" "P2P_PORT" "$((26656 + ((i - 1) * 10)))")"
  NODE_RPC_PORTS[i]="$(get_node_setting "$i" "RPC_PORT" "$((26657 + ((i - 1) * 10)))")"
  NODE_GRPC_PORTS[i]="$(get_node_setting "$i" "GRPC_PORT" "$((9090 + i - 1))")"
  NODE_API_PORTS[i]="$(get_node_setting "$i" "API_PORT" "$((1317 + i - 1))")"
  NODE_WRAPPER_GRPC_ADDRESSES[i]="$(get_node_setting "$i" "WRAPPER_GRPC_ADDRESS" "$DEFAULT_WRAPPER_GRPC_ADDRESS")"
  if [[ -z "${NODE_WRAPPER_GRPC_ADDRESSES[i]}" ]]; then
    echo "missing wrapper gRPC address for node${i}; set WRAPPER_GRPC_ADDRESS, NODE${i}_WRAPPER_GRPC_ADDRESS, or validators[].app.bridge.wrapper_grpc_address in $CHAIN_CONFIG_PATH" >&2
    exit 1
  fi
  NODE_MINA_PUB_KEYS[i]="$(derive_mina_pub_key "${NODE_MINA_PRIV_KEYS[i]}")"
  NODE_GENESIS_FILES[i]="${NODE_HOMES[i]}/config/genesis.json"
done

PRIMARY_NODE_INDEX=1
PRIMARY_HOME="${NODE_HOMES[PRIMARY_NODE_INDEX]}"
PRIMARY_GENESIS_FILE="${NODE_GENESIS_FILES[PRIMARY_NODE_INDEX]}"

echo "==> Cleaning previous homes..."
rm -rf "$LEGACY_CHAIN_HOME" "${NODE_HOMES[@]}"

echo "==> Building binary..."
mkdir -p "$BIN_DIR"
(
  cd "$REPO_ROOT"
  GOFLAGS="$GOFLAGS_WITH_PUREGO" go build -o "$BINARY_PATH" ./cmd/pulsard
)

if [[ "$COMPAT_BINARY_PATH" != "$BINARY_PATH" ]]; then
  cp "$BINARY_PATH" "$COMPAT_BINARY_PATH"
fi

echo "==> Initializing nodes..."
for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  "$BINARY_PATH" init "${NODE_MONIKERS[i]}" --chain-id "$CHAIN_ID" --home "${NODE_HOMES[i]}" >/dev/null 2>&1
done

echo "==> Setting vote extension enable height..."
python3 "$PYTHON_HELPER" set-vote-extension-height \
  --genesis "$PRIMARY_GENESIS_FILE" \
  --height "$VOTE_EXT_ENABLE_HEIGHT"

echo "==> Setting bridge genesis params..."
python3 "$PYTHON_HELPER" patch-bridge-genesis \
  --genesis "$PRIMARY_GENESIS_FILE" \
  --confirmation-depth "$BRIDGE_CONFIRMATION_DEPTH" \
  --contract-address "$BRIDGE_CONTRACT_ADDRESS" \
  --start-block-height "$BRIDGE_START_BLOCK_HEIGHT" \
  --max-block-range "$BRIDGE_MAX_BLOCK_RANGE" \
  --actions-reduced-root-snapshot-window-size "$BRIDGE_ACTIONS_REDUCED_ROOT_SNAPSHOT_WINDOW_SIZE"

echo "==> Creating validator keys..."
for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  "$BINARY_PATH" keys add "${NODE_KEY_NAMES[i]}" --home "${NODE_HOMES[i]}" --keyring-backend "$KEYRING_BACKEND" >/dev/null 2>&1
  NODE_ADDRS[i]="$("$BINARY_PATH" keys show "${NODE_KEY_NAMES[i]}" --address --home "${NODE_HOMES[i]}" --keyring-backend "$KEYRING_BACKEND")"
done

echo "==> Adding genesis accounts..."
for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  "$BINARY_PATH" genesis add-genesis-account "${NODE_ADDRS[i]}" "${STAKE_AMOUNT}${DENOM}" --home "$PRIMARY_HOME" >/dev/null 2>&1
done

for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  if (( i > PRIMARY_NODE_INDEX )); then
    echo "==> Copying shared genesis to ${NODE_MONIKERS[i]}..."
    cp "$PRIMARY_GENESIS_FILE" "${NODE_GENESIS_FILES[i]}"
  fi

  echo "==> Creating gentx for ${NODE_KEY_NAMES[i]}..."
  "$BINARY_PATH" genesis gentx "${NODE_KEY_NAMES[i]}" "${BOND_AMOUNT}${DENOM}" \
    --chain-id "$CHAIN_ID" \
    --home "${NODE_HOMES[i]}" \
    --keyring-backend "$KEYRING_BACKEND" >/dev/null 2>&1
done

echo "==> Collecting gentxs..."
for ((i = PRIMARY_NODE_INDEX + 1; i <= VALIDATOR_COUNT; i++)); do
  cp "${NODE_HOMES[i]}"/config/gentx/*.json "$PRIMARY_HOME/config/gentx/"
done
"$BINARY_PATH" genesis collect-gentxs --home "$PRIMARY_HOME" >/dev/null 2>&1

for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  NODE_COSMOS_PUB_KEYS[i]="$(read_consensus_pub_key "${NODE_HOMES[i]}/config/priv_validator_key.json")"
done

echo "==> Patching keyregistry validator key pairs..."
patch_keyregistry_args=(patch-keyregistry --genesis "$PRIMARY_GENESIS_FILE")
for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  patch_keyregistry_args+=(--cosmos-key "${NODE_COSMOS_PUB_KEYS[i]}")
  patch_keyregistry_args+=(--mina-pub-key "${NODE_MINA_PUB_KEYS[i]}")
done
python3 "$PYTHON_HELPER" "${patch_keyregistry_args[@]}" >/dev/null

echo "==> Validating final genesis..."
"$BINARY_PATH" genesis validate-genesis --home "$PRIMARY_HOME" >/dev/null 2>&1

for ((i = PRIMARY_NODE_INDEX + 1; i <= VALIDATOR_COUNT; i++)); do
  echo "==> Copying final genesis to ${NODE_MONIKERS[i]}..."
  cp "$PRIMARY_GENESIS_FILE" "${NODE_GENESIS_FILES[i]}"
done

for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  NODE_IDS[i]="$("$BINARY_PATH" tendermint show-node-id --home "${NODE_HOMES[i]}")"
done

for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  echo "==> Configuring ${NODE_MONIKERS[i]}..."
  configure_node \
    "${NODE_HOMES[i]}" \
    "${NODE_RPC_PORTS[i]}" \
    "${NODE_P2P_PORTS[i]}" \
    "${NODE_API_PORTS[i]}" \
    "${NODE_GRPC_PORTS[i]}" \
    "$(build_persistent_peers "$i")" \
    "${NODE_MINA_PRIV_KEYS[i]}" \
    "${NODE_MINA_NETWORK_IDS[i]}" \
    "${NODE_WRAPPER_GRPC_ADDRESSES[i]}"
done

echo ""
echo "Setup complete."
echo "  validators:        $VALIDATOR_COUNT"
echo "  binary:            $BINARY_PATH"
echo "  compat binary:     $COMPAT_BINARY_PATH"
for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  echo "  ${NODE_MONIKERS[i]} home:        ${NODE_HOMES[i]}"
  echo "  ${NODE_MONIKERS[i]} address:     ${NODE_ADDRS[i]}"
  echo "  ${NODE_MONIKERS[i]} cosmos key:  ${NODE_COSMOS_PUB_KEYS[i]}"
  echo "  ${NODE_MONIKERS[i]} mina key:    ${NODE_MINA_PUB_KEYS[i]}"
done
echo ""
echo "Start the nodes in separate terminals:"
for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  echo "  $BINARY_PATH start --home ${NODE_HOMES[i]}"
done
echo ""
echo "Validation examples:"
echo "  curl -s http://localhost:${NODE_RPC_PORTS[PRIMARY_NODE_INDEX]}/validators | python3 -m json.tool | grep total"
echo "  $BINARY_PATH query votepersistence vote-extensions --home $PRIMARY_HOME"
echo "  grpcurl -plaintext -d '{\"vote_extension_height\":\"5\"}' localhost:${NODE_GRPC_PORTS[PRIMARY_NODE_INDEX]} pulsarchain.abci.Query/VoteExtBodyByHeight"
echo "  GOFLAGS=\"\${GOFLAGS:-} -tags=purego\" go run ./scripts/devtools verify-vote-extensions --grpc-addr localhost:${NODE_GRPC_PORTS[PRIMARY_NODE_INDEX]}"
