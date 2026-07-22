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
    content = read_text(config_path)
    match = re.search(
        r'bridge:\s*\n\s*wrapper_grpc_address:\s*"([^"]+)"', content
    )
    if not match:
        raise SystemExit(
            f"could not find validators[].app.bridge.wrapper_grpc_address in {config_path}"
        )

    print(match.group(1))
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


def upsert_toml_table(app_toml: str, table_name: str, table_body: str) -> str:
    table_header = f"[{table_name}]"
    table_block = f"{table_header}\n{table_body}\n"

    if f"\n{table_header}\n" in app_toml:
        start = app_toml.index(f"\n{table_header}\n") + 1
        end = app_toml.find("\n[", start + 1)
        if end == -1:
            return app_toml[:start] + table_block

        return app_toml[:start] + table_block + app_toml[end + 1 :]

    return app_toml.rstrip() + f"\n\n{table_block}"


def update_app_config(
    app_path: str,
    min_gas_price: str,
    mina_priv_key: str,
    wrapper_grpc_address: str,
) -> int:
    app_toml = read_text(app_path)
    app_toml = app_toml.replace(
        'minimum-gas-prices = ""',
        f'minimum-gas-prices = "{min_gas_price}"',
        1,
    )

    app_toml = upsert_toml_table(
        app_toml,
        "vote_extension",
        f'priv_key = "{mina_priv_key}"',
    )

    wrapper_grpc_address = wrapper_grpc_address.strip()
    if wrapper_grpc_address:
        app_toml = upsert_toml_table(
            app_toml,
            "bridge",
            f'wrapper_grpc_address = "{wrapper_grpc_address}"',
        )

    write_text(app_path, app_toml)
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Helpers for local testnet setup.")
    subparsers = parser.add_subparsers(dest="command", required=True)

    read_key = subparsers.add_parser("read-mina-priv-key")
    read_key.add_argument("--config", required=True)

    read_wrapper_addr = subparsers.add_parser("read-wrapper-grpc-address")
    read_wrapper_addr.add_argument("--config", required=True)

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

    update_app = subparsers.add_parser("update-app-config")
    update_app.add_argument("--app", required=True)
    update_app.add_argument("--min-gas-price", required=True)
    update_app.add_argument("--mina-priv-key", required=True)
    update_app.add_argument("--wrapper-grpc-address", default="")

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    if args.command == "read-mina-priv-key":
        return read_mina_priv_key(args.config)
    if args.command == "read-wrapper-grpc-address":
        return read_wrapper_grpc_address(args.config)
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
    if args.command == "update-app-config":
        return update_app_config(
            args.app,
            args.min_gas_price,
            args.mina_priv_key,
            args.wrapper_grpc_address,
        )

    parser.print_help(sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
