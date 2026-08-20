#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
PYTHON_HELPER="$SCRIPT_DIR/setup_local_testnet_helper.py"
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
API_BIND_HOST="${API_BIND_HOST:-localhost}"
START_VALIDATORS="${START_VALIDATORS:-0}"
SKIP_BUILD="${SKIP_BUILD:-0}"
SETUP_CONTEXT="${SETUP_CONTEXT:-host}"
RESET_TESTNET="${RESET_TESTNET:-0}"
DEFAULT_VALIDATOR_NAME_PREFIX="validator"
DEFAULT_NODE1_MINA_PRIV_KEY="ES17xFroE2/QOa9yCLXsQ9sJMeIUVwr2ZXcdWGjNLlM="
DEFAULT_NODE2_MINA_PRIV_KEY="PKeRXivUb4gZ/nMKxUK5beEnVJwIrzN71mAf7JVKsng="
DEFAULT_FUNDED_GENESIS_ACCOUNT1_ADDRESS="pulsar1gzr7gqtls7ae9u08f03y04euj9zu4lvtzuusme"
DEFAULT_FUNDED_GENESIS_ACCOUNT1_COINS="1000000000000pmina"
DEFAULT_FUNDED_GENESIS_ACCOUNT2_ADDRESS="pulsar134w5hzxjjs4q7cgppwf9y2wa563jk9dxa65lg5"
DEFAULT_FUNDED_GENESIS_ACCOUNT2_COINS="1000000000000pmina"
FUNDED_GENESIS_ACCOUNT1_ADDRESS="${FUNDED_GENESIS_ACCOUNT1_ADDRESS:-$DEFAULT_FUNDED_GENESIS_ACCOUNT1_ADDRESS}"
FUNDED_GENESIS_ACCOUNT1_COINS="${FUNDED_GENESIS_ACCOUNT1_COINS:-$DEFAULT_FUNDED_GENESIS_ACCOUNT1_COINS}"
FUNDED_GENESIS_ACCOUNT2_ADDRESS="${FUNDED_GENESIS_ACCOUNT2_ADDRESS:-$DEFAULT_FUNDED_GENESIS_ACCOUNT2_ADDRESS}"
FUNDED_GENESIS_ACCOUNT2_COINS="${FUNDED_GENESIS_ACCOUNT2_COINS:-$DEFAULT_FUNDED_GENESIS_ACCOUNT2_COINS}"
DEVTOOLS_BINARY_PATH="${DEVTOOLS_BINARY_PATH:-$BIN_DIR/pulsar-devtools}"

usage() {
  echo "usage: $0 [--reset] [validator-count]" >&2
}

is_truthy() {
  case "$1" in
    1|true|TRUE|True|yes|YES|Yes|on|ON|On) return 0 ;;
    *) return 1 ;;
  esac
}

POSITIONAL_ARGS=()
while (( $# > 0 )); do
  case "$1" in
    --reset)
      RESET_TESTNET="1"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      while (( $# > 0 )); do
        POSITIONAL_ARGS+=("$1")
        shift
      done
      break
      ;;
    -*)
      echo "unknown option: $1" >&2
      usage
      exit 1
      ;;
    *)
      POSITIONAL_ARGS+=("$1")
      ;;
  esac
  shift
done

if (( ${#POSITIONAL_ARGS[@]} > 1 )); then
  usage
  exit 1
fi

VALIDATOR_COUNT="${POSITIONAL_ARGS[0]:-${VALIDATOR_COUNT:-2}}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

directory_has_entries() {
  local path="$1"

  if [[ ! -d "$path" ]]; then
    return 1
  fi

  find "$path" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null | grep -q .
}

derive_mina_pub_key() {
  "$DEVTOOLS_BINARY_PATH" derive-mina-pub "$1"
}

read_consensus_pub_key() {
  python3 "$PYTHON_HELPER" read-consensus-pub-key --priv-validator-key "$1"
}

read_account_pub_key() {
  python3 "$PYTHON_HELPER" read-account-pub-key
}

read_validator_wrapper_config() {
  python3 "$PYTHON_HELPER" read-validator-wrapper-config \
    --config "$1" \
    --index "$2" \
    --key "$3"
}

read_app_wrapper_config() {
  python3 "$PYTHON_HELPER" read-app-wrapper-config --app "$1" --key "$2"
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

read_mina_priv_key() {
  python3 "$PYTHON_HELPER" read-mina-priv-key --config "$1"
}

read_mina_network_id() {
  python3 "$PYTHON_HELPER" read-mina-network-id --config "$1"
}

read_min_gas_price() {
  python3 "$PYTHON_HELPER" read-min-gas-price --app "$1"
}

read_vote_extension_height() {
  python3 "$PYTHON_HELPER" read-vote-extension-height --genesis "$1"
}

read_genesis_chain_id() {
  python3 "$PYTHON_HELPER" read-genesis-chain-id --genesis "$1"
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

resolve_node_wrapper_config() {
  local index="$1"
  local suffix="$2"
  local config_key="$3"
  local global_var_name="$4"
  local node_var_name="NODE${index}_${suffix}"

  if [[ -n "${!node_var_name+x}" ]]; then
    printf '%s\n' "${!node_var_name}"
    return
  fi

  if [[ -n "${!global_var_name+x}" ]]; then
    printf '%s\n' "${!global_var_name}"
    return
  fi

  if [[ -f "$CHAIN_CONFIG_PATH" ]]; then
    read_validator_wrapper_config "$CHAIN_CONFIG_PATH" "$index" "$config_key"
    return
  fi

  printf '\n'
}

default_node_moniker() {
  local index="$1"

  printf '%s%s\n' "$DEFAULT_VALIDATOR_NAME_PREFIX" "$index"
}

default_node_p2p_host() {
  local index="$1"

  # Host-started validators talk over localhost. Docker-generated homes use
  # container names so peers can resolve each other on the Docker network.
  if [[ "$SETUP_CONTEXT" != "container" || "$START_VALIDATORS" == "1" ]]; then
    printf '%s\n' "127.0.0.1"
    return
  fi

  printf '%s%s\n' "$DEFAULT_VALIDATOR_NAME_PREFIX" "$index"
}

use_standard_container_ports() {
  [[ "$SETUP_CONTEXT" == "container" && "$START_VALIDATORS" != "1" ]]
}

default_node_port() {
  local index="$1"
  local base_port="$2"
  local host_offset="$3"

  if use_standard_container_ports; then
    printf '%s\n' "$base_port"
    return
  fi

  printf '%s\n' "$((base_port + ((index - 1) * host_offset)))"
}

configure_node() {
  local home="$1"
  local rpc_port="$2"
  local p2p_port="$3"
  local api_port="$4"
  local grpc_port="$5"
  local pprof_port="$6"
  local persistent_peers="$7"
  local mina_priv_key="$8"
  local mina_network_id="$9"
  local wrapper_grpc_address="${10}"
  local wrapper_grpc_transport_mode="${11}"

  sed -i.bak "s|laddr = \"tcp://127.0.0.1:26657\"|laddr = \"tcp://0.0.0.0:${rpc_port}\"|" "$home/config/config.toml"
  sed -i.bak "s|laddr = \"tcp://0.0.0.0:26656\"|laddr = \"tcp://0.0.0.0:${p2p_port}\"|" "$home/config/config.toml"
  sed -i.bak "s|pprof_laddr = \"localhost:6060\"|pprof_laddr = \"localhost:${pprof_port}\"|" "$home/config/config.toml"
  sed -E -i.bak "s|^persistent_peers = \".*\"|persistent_peers = \"${persistent_peers}\"|" "$home/config/config.toml"
  sed -i.bak 's|addr_book_strict = true|addr_book_strict = false|' "$home/config/config.toml"
  sed -i.bak 's|allow_duplicate_ip = false|allow_duplicate_ip = true|' "$home/config/config.toml"
  sed -E -i.bak 's|^cors_allowed_origins = .*|cors_allowed_origins = ["*"]|' "$home/config/config.toml"

  sed -i.bak "s|address = \"tcp://localhost:1317\"|address = \"tcp://${API_BIND_HOST}:${api_port}\"|" "$home/config/app.toml"
  sed -i.bak "s|address = \"localhost:9090\"|address = \"0.0.0.0:${grpc_port}\"|" "$home/config/app.toml"

  python3 "$PYTHON_HELPER" update-app-config \
    --app "$home/config/app.toml" \
    --min-gas-price "$MIN_GAS_PRICE" \
    --mina-priv-key "$mina_priv_key" \
    --mina-network-id "$mina_network_id" \
    --wrapper-grpc-address "$wrapper_grpc_address" \
    --wrapper-grpc-transport-mode "$wrapper_grpc_transport_mode"
}

grant_wrapper_genesis_access() {
  local home="$1"
  local config_dir="$home/config"
  local genesis_file="$config_dir/genesis.json"

  if [[ "$SETUP_CONTEXT" != "container" ]]; then
    return
  fi

  chgrp 65532 "$home" "$config_dir" "$genesis_file"
  chmod g+x "$home" "$config_dir"
  chmod g+r "$genesis_file"
}

node_home_has_required_files() {
  local index="$1"
  local home="${NODE_HOMES[index]}"

  [[ -f "${NODE_GENESIS_FILES[index]}" ]] &&
    [[ -f "$home/config/priv_validator_key.json" ]] &&
    [[ -f "$home/config/app.toml" ]]
}

node_app_config_matches_expected() {
  local index="$1"
  local app_config="${NODE_HOMES[index]}/config/app.toml"
  local actual_min_gas_price
  local actual_mina_priv_key
  local actual_mina_network_id
  local actual_wrapper_grpc_address
  local actual_wrapper_grpc_transport_mode

  actual_min_gas_price="$(read_min_gas_price "$app_config" 2>/dev/null)" || return 1
  [[ "$actual_min_gas_price" == "$MIN_GAS_PRICE" ]] || return 1

  actual_mina_priv_key="$(read_mina_priv_key "$app_config" 2>/dev/null)" || return 1
  [[ "$actual_mina_priv_key" == "${NODE_MINA_PRIV_KEYS[index]}" ]] || return 1

  actual_mina_network_id="$(read_mina_network_id "$app_config" 2>/dev/null)" || return 1
  [[ "$actual_mina_network_id" == "${NODE_MINA_NETWORK_IDS[index]}" ]] || return 1

  actual_wrapper_grpc_address="$(read_app_wrapper_config "$app_config" "wrapper_grpc_address" 2>/dev/null)" || return 1
  [[ "$actual_wrapper_grpc_address" == "${NODE_WRAPPER_GRPC_ADDRESSES[index]}" ]] || return 1

  actual_wrapper_grpc_transport_mode="$(read_app_wrapper_config "$app_config" "wrapper_grpc_transport_mode" 2>/dev/null)" || return 1
  [[ "$actual_wrapper_grpc_transport_mode" == "${NODE_WRAPPER_GRPC_TRANSPORT_MODES[index]}" ]] || return 1

  # Local testnets run without verifier sidecars. Check the explicit opt-out so
  # homes generated before the validator-first default changed are rebuilt.
  grep -A8 '^\[verification\]$' "$app_config" | grep -q '^enabled = false$' || return 1
  grep -A8 '^\[verification\]$' "$app_config" | grep -q '^grpc_address = ""$' || return 1

  return 0
}

primary_genesis_matches_expected() {
  # TODO: Compare every persisted bridge genesis parameter before production use.
  # The current reuse check does not detect finality, contract, range, or snapshot-window drift.
  local actual_vote_extension_height
  local actual_chain_id
  local i
  local consensus_pub_key
  local -a verify_keyregistry_args=(verify-validator-key-pairs --genesis "$PRIMARY_GENESIS_FILE")

  actual_vote_extension_height="$(read_vote_extension_height "$PRIMARY_GENESIS_FILE" 2>/dev/null)" || return 1
  [[ "$actual_vote_extension_height" == "$VOTE_EXT_ENABLE_HEIGHT" ]] || return 1

  actual_chain_id="$(read_genesis_chain_id "$PRIMARY_GENESIS_FILE" 2>/dev/null)" || return 1
  [[ "$actual_chain_id" == "$CHAIN_ID" ]] || return 1

  for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
    consensus_pub_key="$(read_consensus_pub_key "${NODE_HOMES[i]}/config/priv_validator_key.json" 2>/dev/null)" || return 1
    verify_keyregistry_args+=(--cosmos-key "$consensus_pub_key")
  done

  python3 "$PYTHON_HELPER" "${verify_keyregistry_args[@]}" >/dev/null 2>&1
}

testnet_state_is_complete() {
  local i

  for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
    node_home_has_required_files "$i" || return 1
    node_app_config_matches_expected "$i" || return 1
  done

  primary_genesis_matches_expected || return 1

  for ((i = PRIMARY_NODE_INDEX + 1; i <= VALIDATOR_COUNT; i++)); do
    cmp -s "$PRIMARY_GENESIS_FILE" "${NODE_GENESIS_FILES[i]}" || return 1
  done

  return 0
}

build_persistent_peers() {
  local current_index="$1"
  local i
  local -a peers=()

  for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
    if (( i == current_index )); then
      continue
    fi

    peers+=("${NODE_IDS[i]}@${NODE_P2P_HOSTS[i]}:${NODE_P2P_PORTS[i]}")
  done

  local IFS=,
  printf '%s\n' "${peers[*]}"
}

cleanup_validators() {
  trap - EXIT INT TERM

  if (( ${#VALIDATOR_PIDS[@]} == 0 )); then
    return
  fi

  kill "${VALIDATOR_PIDS[@]}" 2>/dev/null || true
  wait "${VALIDATOR_PIDS[@]}" 2>/dev/null || true
}

monitor_validators() {
  local pid

  while :; do
    for pid in "${VALIDATOR_PIDS[@]}"; do
      if ! kill -0 "$pid" 2>/dev/null; then
        wait "$pid"
        return $?
      fi
    done

    sleep 1
  done
}

require_cmd python3

if ! [[ "$VALIDATOR_COUNT" =~ ^[0-9]+$ ]] || (( VALIDATOR_COUNT < 1 )); then
  echo "validator count must be a positive integer, got: $VALIDATOR_COUNT" >&2
  exit 1
fi

declare -a NODE_HOMES NODE_MONIKERS NODE_KEY_NAMES NODE_MINA_PRIV_KEYS NODE_MINA_PUB_KEYS
declare -a NODE_MINA_NETWORK_IDS NODE_P2P_HOSTS
declare -a NODE_P2P_PORTS NODE_RPC_PORTS NODE_GRPC_PORTS NODE_API_PORTS NODE_PPROF_PORTS
declare -a NODE_GENESIS_FILES NODE_ADDRS NODE_COSMOS_PUB_KEYS NODE_IDS
declare -a NODE_WRAPPER_GRPC_ADDRESSES NODE_WRAPPER_GRPC_TRANSPORT_MODES

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
  NODE_MONIKERS[i]="$(get_node_setting "$i" "MONIKER" "$(default_node_moniker "$i")")"
  NODE_KEY_NAMES[i]="$(get_node_setting "$i" "KEY_NAME" "validator${i}")"
  NODE_MINA_PRIV_KEYS[i]="$(get_node_setting "$i" "MINA_PRIV_KEY" "$(default_node_mina_priv_key "$i")")"
  validate_mina_priv_key "$i" "${NODE_MINA_PRIV_KEYS[i]}"
  NODE_MINA_NETWORK_IDS[i]="$(get_node_setting "$i" "MINA_NETWORK_ID" "$DEFAULT_MINA_NETWORK_ID")"
  validate_mina_network_id "$i" "${NODE_MINA_NETWORK_IDS[i]}"
  NODE_P2P_HOSTS[i]="$(get_node_setting "$i" "P2P_HOST" "$(default_node_p2p_host "$i")")"
  NODE_P2P_PORTS[i]="$(get_node_setting "$i" "P2P_PORT" "$(default_node_port "$i" 26656 10)")"
  NODE_RPC_PORTS[i]="$(get_node_setting "$i" "RPC_PORT" "$(default_node_port "$i" 26657 10)")"
  NODE_GRPC_PORTS[i]="$(get_node_setting "$i" "GRPC_PORT" "$(default_node_port "$i" 9090 1)")"
  NODE_API_PORTS[i]="$(get_node_setting "$i" "API_PORT" "$(default_node_port "$i" 1317 1)")"
  NODE_PPROF_PORTS[i]="$(get_node_setting "$i" "PPROF_PORT" "$(default_node_port "$i" 6060 1)")"
  NODE_WRAPPER_GRPC_ADDRESSES[i]="$(resolve_node_wrapper_config "$i" "WRAPPER_GRPC_ADDRESS" "wrapper_grpc_address" "WRAPPER_GRPC_ADDRESS")"
  if [[ -z "${NODE_WRAPPER_GRPC_ADDRESSES[i]}" ]]; then
    echo "missing wrapper gRPC address for node${i}; set WRAPPER_GRPC_ADDRESS, NODE${i}_WRAPPER_GRPC_ADDRESS, or validators[].app.bridge.wrapper_grpc_address in $CHAIN_CONFIG_PATH" >&2
    exit 1
  fi
  NODE_WRAPPER_GRPC_TRANSPORT_MODES[i]="$(resolve_node_wrapper_config "$i" "WRAPPER_GRPC_TRANSPORT_MODE" "wrapper_grpc_transport_mode" "WRAPPER_GRPC_TRANSPORT_MODE")"
  case "${NODE_WRAPPER_GRPC_TRANSPORT_MODES[i]}" in
    loopback | trusted-network) ;;
    "")
      echo "missing wrapper gRPC transport mode for node${i}; set WRAPPER_GRPC_TRANSPORT_MODE, NODE${i}_WRAPPER_GRPC_TRANSPORT_MODE, or validators[].app.bridge.wrapper_grpc_transport_mode in $CHAIN_CONFIG_PATH" >&2
      exit 1
      ;;
    *)
      echo "invalid wrapper gRPC transport mode for node${i}: ${NODE_WRAPPER_GRPC_TRANSPORT_MODES[i]}" >&2
      exit 1
      ;;
  esac
  NODE_GENESIS_FILES[i]="${NODE_HOMES[i]}/config/genesis.json"
done

PRIMARY_NODE_INDEX=1
PRIMARY_HOME="${NODE_HOMES[PRIMARY_NODE_INDEX]}"
PRIMARY_GENESIS_FILE="${NODE_GENESIS_FILES[PRIMARY_NODE_INDEX]}"

existing_node_homes=0
for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  if directory_has_entries "${NODE_HOMES[i]}"; then
    ((existing_node_homes += 1))
  fi
done

if (( existing_node_homes > 0 )); then
  if is_truthy "$RESET_TESTNET"; then
    echo "==> Resetting existing validator homes..."
    rm -rf "$LEGACY_CHAIN_HOME" "${NODE_HOMES[@]}"
  elif (( existing_node_homes == VALIDATOR_COUNT )) && testnet_state_is_complete; then
    echo "==> Existing testnet detected; leaving validator homes unchanged."
    echo "    Use --reset or RESET_TESTNET=1 to recreate the validator state."
    exit 0
  else
    echo "partial validator state detected under the requested homes:" >&2
    for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
      if [[ -e "${NODE_HOMES[i]}" ]]; then
        echo "  - ${NODE_HOMES[i]}" >&2
      fi
    done
    echo "rerun with --reset (or RESET_TESTNET=1) to recreate the testnet safely" >&2
    exit 1
  fi
fi

if [[ "$SKIP_BUILD" == "1" ]]; then
  if [[ ! -x "$BINARY_PATH" ]]; then
    echo "expected prebuilt binary at $BINARY_PATH" >&2
    exit 1
  fi
  if [[ ! -x "$DEVTOOLS_BINARY_PATH" ]]; then
    echo "expected prebuilt devtools binary at $DEVTOOLS_BINARY_PATH" >&2
    exit 1
  fi
  echo "==> Reusing prebuilt binaries at $BINARY_PATH and $DEVTOOLS_BINARY_PATH..."
else
  require_cmd go

  echo "==> Building binary..."
  mkdir -p "$BIN_DIR" "$(dirname "$COMPAT_BINARY_PATH")" "$(dirname "$DEVTOOLS_BINARY_PATH")"
  (
    cd "$REPO_ROOT"
    GOFLAGS="$GOFLAGS_WITH_PUREGO" go build -o "$BINARY_PATH" ./cmd/pulsard
    GOFLAGS="$GOFLAGS_WITH_PUREGO" go build -o "$DEVTOOLS_BINARY_PATH" ./scripts/devtools
  )

  if [[ "$COMPAT_BINARY_PATH" != "$BINARY_PATH" ]]; then
    cp "$BINARY_PATH" "$COMPAT_BINARY_PATH"
  fi
fi

for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  NODE_MINA_PUB_KEYS[i]="$(derive_mina_pub_key "${NODE_MINA_PRIV_KEYS[i]}")"
done

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

E2E_USER_MINA_PUB_KEY=""
E2E_USER_COSMOS_PUB_KEY=""
if [[ -n "${E2E_USER_MINA_PRIV_KEY:-}" ]]; then
  validate_mina_priv_key "e2e-user" "$E2E_USER_MINA_PRIV_KEY"
  E2E_USER_MINA_PUB_KEY="$(derive_mina_pub_key "$E2E_USER_MINA_PRIV_KEY")"
  E2E_USER_COSMOS_PUB_KEY="$(
    "$BINARY_PATH" keys show "${NODE_KEY_NAMES[PRIMARY_NODE_INDEX]}" \
      --pubkey \
      --home "$PRIMARY_HOME" \
      --keyring-backend "$KEYRING_BACKEND" | read_account_pub_key
  )"
fi

echo "==> Adding genesis accounts..."
for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  "$BINARY_PATH" genesis add-genesis-account "${NODE_ADDRS[i]}" "${STAKE_AMOUNT}${DENOM}" --home "$PRIMARY_HOME" >/dev/null 2>&1
done

FUNDED_GENESIS_ACCOUNT_ADDRESSES=(
  "$FUNDED_GENESIS_ACCOUNT1_ADDRESS"
  "$FUNDED_GENESIS_ACCOUNT2_ADDRESS"
)
FUNDED_GENESIS_ACCOUNT_COINS=(
  "$FUNDED_GENESIS_ACCOUNT1_COINS"
  "$FUNDED_GENESIS_ACCOUNT2_COINS"
)
for ((i = 0; i < ${#FUNDED_GENESIS_ACCOUNT_ADDRESSES[@]}; i++)); do
  validate_non_empty "funded genesis account $((i + 1)) address" "${FUNDED_GENESIS_ACCOUNT_ADDRESSES[i]}"
  validate_non_empty "funded genesis account $((i + 1)) coins" "${FUNDED_GENESIS_ACCOUNT_COINS[i]}"
  "$BINARY_PATH" genesis add-genesis-account \
    "${FUNDED_GENESIS_ACCOUNT_ADDRESSES[i]}" \
    "${FUNDED_GENESIS_ACCOUNT_COINS[i]}" \
    --home "$PRIMARY_HOME" >/dev/null 2>&1
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
    --keyring-backend "$KEYRING_BACKEND"
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
if [[ -n "$E2E_USER_MINA_PUB_KEY" ]]; then
  patch_keyregistry_args+=(--user-mina-pub-key "$E2E_USER_MINA_PUB_KEY")
  patch_keyregistry_args+=(--user-cosmos-pub-key "$E2E_USER_COSMOS_PUB_KEY")
fi
python3 "$PYTHON_HELPER" "${patch_keyregistry_args[@]}" >/dev/null

echo "==> Validating final genesis..."
"$BINARY_PATH" genesis validate-genesis --home "$PRIMARY_HOME" >/dev/null 2>&1

for ((i = PRIMARY_NODE_INDEX + 1; i <= VALIDATOR_COUNT; i++)); do
  echo "==> Copying final genesis to ${NODE_MONIKERS[i]}..."
  cp "$PRIMARY_GENESIS_FILE" "${NODE_GENESIS_FILES[i]}"
done

for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  grant_wrapper_genesis_access "${NODE_HOMES[i]}"
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
    "${NODE_PPROF_PORTS[i]}" \
    "$(build_persistent_peers "$i")" \
    "${NODE_MINA_PRIV_KEYS[i]}" \
    "${NODE_MINA_NETWORK_IDS[i]}" \
    "${NODE_WRAPPER_GRPC_ADDRESSES[i]}" \
    "${NODE_WRAPPER_GRPC_TRANSPORT_MODES[i]}"
done

echo ""
echo "Setup complete."
echo "  validators:        $VALIDATOR_COUNT"
if [[ "$SETUP_CONTEXT" != "container" ]]; then
  echo "  binary:            $BINARY_PATH"
  echo "  compat binary:     $COMPAT_BINARY_PATH"
fi
for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
  echo "  ${NODE_MONIKERS[i]} home:        ${NODE_HOMES[i]}"
  echo "  ${NODE_MONIKERS[i]} address:     ${NODE_ADDRS[i]}"
  echo "  ${NODE_MONIKERS[i]} cosmos key:  ${NODE_COSMOS_PUB_KEYS[i]}"
  echo "  ${NODE_MONIKERS[i]} mina key:    ${NODE_MINA_PUB_KEYS[i]}"
done
echo ""
if [[ "$START_VALIDATORS" == "1" ]]; then
  echo "Validators will now be started automatically."
else
  if [[ "$SETUP_CONTEXT" == "container" ]]; then
    echo "Validator homes were generated in their mounted Docker volumes."
  else
    echo "Start the nodes in separate terminals:"
    for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
      echo "  $BINARY_PATH start --home ${NODE_HOMES[i]}"
    done
  fi
fi
if [[ "$SETUP_CONTEXT" != "container" ]]; then
  echo ""
  echo "Validation examples:"
  echo "  curl -s http://localhost:${NODE_RPC_PORTS[PRIMARY_NODE_INDEX]}/validators | python3 -m json.tool | grep total"
  echo "  grpcurl -plaintext -d '{\"vote_extension_height\":\"5\"}' localhost:${NODE_GRPC_PORTS[PRIMARY_NODE_INDEX]} pulsarchain.abci.Query/VoteExtBodyByHeight"
  echo "  $BINARY_PATH query votepersistence vote-extensions --home $PRIMARY_HOME"
  echo "  $DEVTOOLS_BINARY_PATH verify-vote-extensions --grpc-addr localhost:${NODE_GRPC_PORTS[PRIMARY_NODE_INDEX]}"
fi

if [[ "$START_VALIDATORS" == "1" ]]; then
  declare -a VALIDATOR_PIDS=()

  trap cleanup_validators EXIT
  trap 'cleanup_validators; exit 130' INT TERM

  echo ""
  echo "==> Starting validators..."
  for ((i = 1; i <= VALIDATOR_COUNT; i++)); do
    echo "  ${NODE_MONIKERS[i]}: $BINARY_PATH start --home ${NODE_HOMES[i]}"
    "$BINARY_PATH" start --home "${NODE_HOMES[i]}" &
    VALIDATOR_PIDS+=("$!")
  done

  monitor_validators
fi
