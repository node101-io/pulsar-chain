#!/usr/bin/env python3

import argparse
import base64
import hashlib
import json
import re
import sys
from pathlib import Path
from typing import Optional


MINA_SCALAR_FIELD = int(
    "40000000000000000000000000000000224698fc0994a8dd8c46eb2100000001", 16
)


def read_text(path_str: str) -> str:
    return Path(path_str).read_text(encoding="utf-8")


def write_text(path_str: str, content: str) -> None:
    Path(path_str).write_text(content, encoding="utf-8")


def read_json(path_str: str):
    return json.loads(read_text(path_str))


def write_json(path_str: str, payload) -> None:
    write_text(path_str, json.dumps(payload, indent=2) + "\n")


def read_nested_config_value(
    config_path: str, section_name: str, key_name: str
) -> Optional[str]:
    lines = read_text(config_path).splitlines()
    key_pattern = re.compile(
        rf'^\s*{re.escape(key_name)}:\s*(?:"([^"]*)"|\'([^\']*)\'|([^#\n]+?))\s*$'
    )

    for index, line in enumerate(lines):
        section_match = re.match(rf"^(\s*){re.escape(section_name)}:\s*$", line)
        if not section_match:
            continue

        section_indent = len(section_match.group(1))
        for nested_line in lines[index + 1 :]:
            if not nested_line.strip():
                continue

            nested_indent = len(nested_line) - len(nested_line.lstrip(" "))
            if nested_indent <= section_indent:
                break

            key_match = key_pattern.match(nested_line)
            if key_match:
                for group in key_match.groups():
                    if group is not None:
                        return group.strip()
                return ""

    return None


def read_mina_priv_key(config_path: str) -> int:
    content = read_text(config_path)
    match = re.search(r'vote_extension:\s*\n\s*priv_key:\s*"([^"]+)"', content)
    if not match:
        raise SystemExit(
            f"could not find validators[].app.vote_extension.priv_key in {config_path}"
        )

    print(match.group(1))
    return 0


def read_wrapper_grpc_address(config_path: str) -> int:
    wrapper_grpc_address = read_nested_config_value(
        config_path, "bridge", "wrapper_grpc_address"
    )
    if wrapper_grpc_address is None:
        print("")
        return 0

    print(wrapper_grpc_address)
    return 0


def read_scalar_in_block(
    lines: list[str],
    start_index: int,
    end_index: int,
    parent_indent: int,
    key_name: str,
) -> Optional[str]:
    key_pattern = re.compile(
        rf'^\s*{re.escape(key_name)}:\s*(?:"([^"]*)"|\'([^\']*)\'|([^#\n]+?))\s*$'
    )

    for index in range(start_index, end_index):
        line = lines[index]
        if not line.strip():
            continue

        indent = len(line) - len(line.lstrip(" "))
        if indent <= parent_indent:
            break

        key_match = key_pattern.match(line)
        if not key_match:
            continue

        for group in key_match.groups():
            if group is not None:
                return group.strip()
        return ""

    return None


def find_named_block(
    lines: list[str],
    start_index: int,
    end_index: int,
    parent_indent: int,
    block_name: str,
) -> Optional[tuple[int, int, int]]:
    block_pattern = re.compile(rf"^(\s*){re.escape(block_name)}:\s*$")

    for index in range(start_index, end_index):
        line = lines[index]
        if not line.strip():
            continue

        indent = len(line) - len(line.lstrip(" "))
        if indent <= parent_indent:
            break

        block_match = block_pattern.match(line)
        if not block_match:
            continue

        block_indent = len(block_match.group(1))
        block_end = end_index
        for nested_index in range(index + 1, end_index):
            nested_line = lines[nested_index]
            if not nested_line.strip():
                continue

            nested_indent = len(nested_line) - len(nested_line.lstrip(" "))
            if nested_indent <= block_indent:
                block_end = nested_index
                break

        return index + 1, block_end, block_indent

    return None


def read_config_path_value(config_path: str, path: list[str]) -> Optional[str]:
    lines = read_text(config_path).splitlines()
    start_index = 0
    end_index = len(lines)
    parent_indent = -1

    for block_name in path[:-1]:
        block = find_named_block(
            lines, start_index, end_index, parent_indent, block_name
        )
        if block is None:
            return None

        start_index, end_index, parent_indent = block

    return read_scalar_in_block(lines, start_index, end_index, parent_indent, path[-1])


def read_bridge_genesis_param(config_path: str, key_name: str) -> int:
    value = read_config_path_value(
        config_path,
        ["genesis", "app_state", "bridge", "params", key_name],
    )
    if value is None:
        print("")
        return 0

    print(value)
    return 0


def read_mina_network_id(config_path: str) -> int:
    network_id = read_nested_config_value(config_path, "mina", "network_id")
    if network_id is None:
        raise SystemExit(
            f"could not find validators[].app.mina.network_id in {config_path}"
        )

    print(network_id)
    return 0


def set_vote_extension_height(genesis_path: str, height: str) -> int:
    genesis = read_json(genesis_path)
    genesis["consensus"]["params"]["abci"]["vote_extensions_enable_height"] = height
    write_json(genesis_path, genesis)
    return 0


def read_consensus_pub_key(priv_validator_key_path: str) -> int:
    priv_validator_key = read_json(priv_validator_key_path)
    print(priv_validator_key["pub_key"]["value"])
    return 0


def generate_default_mina_priv_key(index: str) -> int:
    data = hashlib.sha256(f"pulsar-local-testnet-node-{index}".encode()).digest()

    while True:
        value = int.from_bytes(data, "big") % MINA_SCALAR_FIELD
        if value != 0:
            print(base64.b64encode(value.to_bytes(32, "big")).decode())
            return 0

        data = hashlib.sha256(data).digest()


def validate_mina_priv_key(index: str, mina_priv_key: str) -> int:
    try:
        raw = base64.b64decode(mina_priv_key, validate=True)
    except Exception as exc:
        raise SystemExit(f"node{index} mina private key is not valid base64: {exc}")

    if len(raw) != 32:
        raise SystemExit(
            f"node{index} mina private key must decode to 32 bytes, got {len(raw)}"
        )

    value = int.from_bytes(raw, "big")
    if value == 0 or value >= MINA_SCALAR_FIELD:
        raise SystemExit(
            "node"
            f"{index} mina private key must represent a non-zero scalar smaller than "
            f"{MINA_SCALAR_FIELD:x}"
        )

    return 0


def extract_gentx_consensus_pub_keys(genesis) -> list[str]:
    gentxs = genesis["app_state"]["genutil"]["gen_txs"]
    cosmos_keys = []

    for gentx in gentxs:
        messages = gentx.get("body", {}).get("messages", [])
        if not messages:
            continue

        pub_key = messages[0].get("pubkey", {}).get("key")
        if pub_key:
            cosmos_keys.append(pub_key)

    return cosmos_keys


def patch_keyregistry(
    genesis_path: str, mina_pub_keys: list[str], cosmos_keys: Optional[list[str]] = None
) -> int:
    genesis = read_json(genesis_path)

    if not mina_pub_keys:
        raise SystemExit("at least one --mina-pub-key is required")

    if cosmos_keys is None:
        cosmos_keys = extract_gentx_consensus_pub_keys(genesis)

    if len(cosmos_keys) != len(mina_pub_keys):
        raise SystemExit(
            "validator key pair count mismatch: "
            f"{len(cosmos_keys)} cosmos keys for {len(mina_pub_keys)} mina keys"
        )

    keyregistry = genesis["app_state"].setdefault("keyregistry", {})
    keyregistry["params"] = keyregistry.get("params", {})
    keyregistry["user_key_pairs"] = []
    keyregistry["validator_key_pairs"] = [
        {
            "cosmos_key": cosmos_key,
            "mina_key": mina_pub_key,
        }
        for cosmos_key, mina_pub_key in zip(cosmos_keys, mina_pub_keys)
    ]

    write_json(genesis_path, genesis)
    print("\n".join(cosmos_keys))
    return 0


def patch_bridge_genesis(
    genesis_path: str,
    confirmation_depth: str,
    contract_address: str,
    start_block_height: str,
    max_block_range: str,
) -> int:
    try:
        confirmation_depth_int = int(confirmation_depth)
    except ValueError as exc:
        raise SystemExit(f"invalid confirmation depth: {confirmation_depth}") from exc

    if confirmation_depth_int <= 0:
        raise SystemExit("confirmation depth must be greater than 0")

    contract_address = contract_address.strip()
    if not contract_address:
        raise SystemExit("contract address must not be empty")

    try:
        start_block_height_int = int(start_block_height)
    except ValueError as exc:
        raise SystemExit(f"invalid start block height: {start_block_height}") from exc

    if start_block_height_int <= 0:
        raise SystemExit("start block height must be greater than 0")

    try:
        max_block_range_int = int(max_block_range)
    except ValueError as exc:
        raise SystemExit(f"invalid max block range: {max_block_range}") from exc

    if max_block_range_int <= 0:
        raise SystemExit("max block range must be greater than 0")

    genesis = read_json(genesis_path)
    app_state = genesis.setdefault("app_state", {})
    bridge = app_state.setdefault("bridge", {})
    bridge["params"] = {
        "confirmation_depth": str(confirmation_depth_int),
        "contract_address": contract_address,
        "start_block_height": str(start_block_height_int),
        "max_block_range": str(max_block_range_int),
    }

    bridge["bridge_state"] = {
        "latest_fetched_mina_height": str(start_block_height_int - 1),
    }

    write_json(genesis_path, genesis)
    return 0


def upsert_toml_key(
    app_toml: str,
    table_name: str,
    key_name: str,
    value: str,
    *,
    quote_value: bool = True,
) -> str:
    table_header = f"[{table_name}]"
    if quote_value:
        key_line = f'{key_name} = "{value}"'
    else:
        key_line = f"{key_name} = {value}"
    lines = app_toml.rstrip("\n").split("\n")

    table_start = None
    for index, line in enumerate(lines):
        if line.strip() == table_header:
            table_start = index
            break

    if table_start is None:
        if lines and lines[-1].strip():
            lines.extend(["", table_header, key_line])
        else:
            lines.extend([table_header, key_line])
        return "\n".join(lines) + "\n"

    table_end = len(lines)
    for index in range(table_start + 1, len(lines)):
        stripped = lines[index].strip()
        if stripped.startswith("[") and stripped.endswith("]"):
            table_end = index
            break

    key_pattern = re.compile(rf"^\s*{re.escape(key_name)}\s*=")
    for index in range(table_start + 1, table_end):
        if key_pattern.match(lines[index]):
            lines[index] = key_line
            break
    else:
        while table_end > table_start + 1 and not lines[table_end - 1].strip():
            table_end -= 1
        lines.insert(table_end, key_line)

    return "\n".join(lines) + "\n"


def update_app_config(
    app_path: str,
    min_gas_price: str,
    mina_priv_key: str,
    mina_network_id: str,
    wrapper_grpc_address: str,
) -> int:
    app_toml = read_text(app_path)
    app_toml = app_toml.replace(
        'minimum-gas-prices = ""',
        f'minimum-gas-prices = "{min_gas_price}"',
        1,
    )

    app_toml = upsert_toml_key(app_toml, "vote_extension", "priv_key", mina_priv_key)

    app_toml = upsert_toml_key(app_toml, "mina", "network_id", mina_network_id)

    wrapper_grpc_address = wrapper_grpc_address.strip()
    if wrapper_grpc_address:
        app_toml = upsert_toml_key(
            app_toml, "bridge", "wrapper_grpc_address", wrapper_grpc_address
        )

    write_text(app_path, app_toml)
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Helpers for local testnet setup.")
    subparsers = parser.add_subparsers(dest="command", required=True)

    read_key = subparsers.add_parser("read-mina-priv-key")
    read_key.add_argument("--config", required=True)

    read_network_id = subparsers.add_parser("read-mina-network-id")
    read_network_id.add_argument("--config", required=True)

    read_wrapper_addr = subparsers.add_parser("read-wrapper-grpc-address")
    read_wrapper_addr.add_argument("--config", required=True)

    read_bridge_param_cmd = subparsers.add_parser("read-bridge-genesis-param")
    read_bridge_param_cmd.add_argument("--config", required=True)
    read_bridge_param_cmd.add_argument("--key", required=True)

    read_consensus_key = subparsers.add_parser("read-consensus-pub-key")
    read_consensus_key.add_argument("--priv-validator-key", required=True)

    set_height = subparsers.add_parser("set-vote-extension-height")
    set_height.add_argument("--genesis", required=True)
    set_height.add_argument("--height", required=True)

    generate_mina_key = subparsers.add_parser("generate-default-mina-priv-key")
    generate_mina_key.add_argument("--index", required=True)

    validate_mina_key = subparsers.add_parser("validate-mina-priv-key")
    validate_mina_key.add_argument("--index", required=True)
    validate_mina_key.add_argument("--mina-priv-key", required=True)

    patch_registry = subparsers.add_parser("patch-keyregistry")
    patch_registry.add_argument("--genesis", required=True)
    patch_registry.add_argument("--cosmos-key", action="append")
    patch_registry.add_argument("--mina-pub-key", action="append", required=True)

    patch_bridge = subparsers.add_parser("patch-bridge-genesis")
    patch_bridge.add_argument("--genesis", required=True)
    patch_bridge.add_argument("--confirmation-depth", required=True)
    patch_bridge.add_argument("--contract-address", required=True)
    patch_bridge.add_argument("--start-block-height", required=True)
    patch_bridge.add_argument("--max-block-range", required=True)

    update_app = subparsers.add_parser("update-app-config")
    update_app.add_argument("--app", required=True)
    update_app.add_argument("--min-gas-price", required=True)
    update_app.add_argument("--mina-priv-key", required=True)
    update_app.add_argument("--mina-network-id", required=True)
    update_app.add_argument("--wrapper-grpc-address", default="")

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    if args.command == "read-mina-priv-key":
        return read_mina_priv_key(args.config)
    if args.command == "read-wrapper-grpc-address":
        return read_wrapper_grpc_address(args.config)
    if args.command == "read-bridge-genesis-param":
        return read_bridge_genesis_param(args.config, args.key)
    if args.command == "read-mina-network-id":
        return read_mina_network_id(args.config)
    if args.command == "read-consensus-pub-key":
        return read_consensus_pub_key(args.priv_validator_key)
    if args.command == "set-vote-extension-height":
        return set_vote_extension_height(args.genesis, args.height)
    if args.command == "generate-default-mina-priv-key":
        return generate_default_mina_priv_key(args.index)
    if args.command == "validate-mina-priv-key":
        return validate_mina_priv_key(args.index, args.mina_priv_key)
    if args.command == "patch-keyregistry":
        return patch_keyregistry(args.genesis, args.mina_pub_key, args.cosmos_key)
    if args.command == "patch-bridge-genesis":
        return patch_bridge_genesis(
            args.genesis,
            args.confirmation_depth,
            args.contract_address,
            args.start_block_height,
            args.max_block_range,
        )
    if args.command == "update-app-config":
        return update_app_config(
            args.app,
            args.min_gas_price,
            args.mina_priv_key,
            args.mina_network_id,
            args.wrapper_grpc_address,
        )

    parser.print_help(sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
