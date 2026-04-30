#!/usr/bin/env python3

import argparse
import json
import re
import sys
from pathlib import Path


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


def set_vote_extension_height(genesis_path: str, height: str) -> int:
    genesis = read_json(genesis_path)
    genesis["consensus"]["params"]["abci"]["vote_extensions_enable_height"] = height
    write_json(genesis_path, genesis)
    return 0


def patch_keyregistry(genesis_path: str, mina_pub_key: str) -> int:
    genesis = read_json(genesis_path)

    cosmos_pub_key = (
        genesis["app_state"]["genutil"]["gen_txs"][0]["body"]["messages"][0]["pubkey"]["key"]
    )

    keyregistry = genesis["app_state"].setdefault("keyregistry", {})
    keyregistry["params"] = keyregistry.get("params", {})
    keyregistry["user_key_pairs"] = []
    keyregistry["validator_key_pairs"] = [
        {
            "cosmos_key": cosmos_pub_key,
            "mina_key": mina_pub_key,
        }
    ]

    write_json(genesis_path, genesis)
    print(cosmos_pub_key)
    return 0


def update_app_config(app_path: str, min_gas_price: str, mina_priv_key: str) -> int:
    app_toml = read_text(app_path)
    app_toml = app_toml.replace(
        'minimum-gas-prices = ""',
        f'minimum-gas-prices = "{min_gas_price}"',
        1,
    )

    vote_extension_block = f'[vote_extension]\npriv_key = "{mina_priv_key}"\n'

    if "\n[vote_extension]\n" in app_toml:
        start = app_toml.index("\n[vote_extension]\n") + 1
        end = app_toml.find("\n[", start + 1)
        if end == -1:
            app_toml = app_toml[:start] + vote_extension_block
        else:
            app_toml = app_toml[:start] + vote_extension_block + app_toml[end + 1 :]
    else:
        app_toml = app_toml.rstrip() + f"\n\n{vote_extension_block}"

    write_text(app_path, app_toml)
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Helpers for local testnet setup.")
    subparsers = parser.add_subparsers(dest="command", required=True)

    read_key = subparsers.add_parser("read-mina-priv-key")
    read_key.add_argument("--config", required=True)

    set_height = subparsers.add_parser("set-vote-extension-height")
    set_height.add_argument("--genesis", required=True)
    set_height.add_argument("--height", required=True)

    patch_registry = subparsers.add_parser("patch-keyregistry")
    patch_registry.add_argument("--genesis", required=True)
    patch_registry.add_argument("--mina-pub-key", required=True)

    update_app = subparsers.add_parser("update-app-config")
    update_app.add_argument("--app", required=True)
    update_app.add_argument("--min-gas-price", required=True)
    update_app.add_argument("--mina-priv-key", required=True)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    if args.command == "read-mina-priv-key":
        return read_mina_priv_key(args.config)
    if args.command == "set-vote-extension-height":
        return set_vote_extension_height(args.genesis, args.height)
    if args.command == "patch-keyregistry":
        return patch_keyregistry(args.genesis, args.mina_pub_key)
    if args.command == "update-app-config":
        return update_app_config(args.app, args.min_gas_price, args.mina_priv_key)

    parser.print_help(sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
