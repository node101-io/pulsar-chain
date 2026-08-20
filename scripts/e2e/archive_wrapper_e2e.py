#!/usr/bin/env python3

import argparse
import json
import sys
from pathlib import Path
from typing import Optional


POSTGRES_IMAGE = "postgres:17-bookworm@sha256:4f736ae292687621d4dbe0d499ffd024a36bd2ee7d8ca6f2ccd4c800f047b394"


def render_postgres_compose(
    output: str, schema: str, seed: str, network_key: Optional[str]
) -> int:
    compose = {
        "services": {
            "postgres": {
                "image": POSTGRES_IMAGE,
                "environment": {
                    "POSTGRES_DB": "archive",
                    "POSTGRES_USER": "archive",
                    "POSTGRES_PASSWORD": "e2e-secret",
                },
                "healthcheck": {
                    "test": ["CMD-SHELL", "pg_isready -U archive -d archive"],
                    "interval": "2s",
                    "timeout": "2s",
                    "retries": 30,
                },
                "volumes": [
                    f"{Path(schema).resolve()}:/docker-entrypoint-initdb.d/01-schema.sql:ro",
                    f"{Path(seed).resolve()}:/docker-entrypoint-initdb.d/02-seed.sql:ro",
                ],
            }
        }
    }
    if network_key:
        compose["services"]["postgres"]["networks"] = [network_key]
    Path(output).write_text(json.dumps(compose, indent=2) + "\n", encoding="utf-8")
    return 0


def render_wrapper_config(output: str, network_id: str) -> int:
    config = {
        "block_height_database_key": "archive-wrapper",
        "db_path": "/var/lib/archive-wrapper/data/leveldb",
        "grpc_listen_address": "0.0.0.0:9095",
        "grpc_transport_mode": "trusted-network",
        "control_socket_path": "/run/archive-wrapper/control.sock",
        "deployment_metadata_key": "archive-wrapper:deployment",
        "deployment_metadata": {
            "schema_version": 1,
            "mina_network_id": network_id,
        },
    }
    Path(output).write_text(json.dumps(config, indent=2) + "\n", encoding="utf-8")
    return 0


def read_json(path: str):
    if path == "-":
        return json.load(sys.stdin)
    return json.loads(Path(path).read_text(encoding="utf-8"))


def json_value(path: str, field_path: str, default: Optional[str] = None) -> int:
    value = read_json(path)
    try:
        for field in field_path.split("."):
            if isinstance(value, list):
                value = value[int(field)]
            else:
                value = value[field]
    except (IndexError, KeyError):
        if default is None:
            raise
        value = default
    if isinstance(value, (dict, list)):
        print(json.dumps(value, sort_keys=True))
    elif isinstance(value, bool):
        print(str(value).lower())
    else:
        print(value)
    return 0


def bank_balance(path: str, denom: str) -> int:
    payload = read_json(path)
    balances = payload.get("balances")
    if balances is None and payload.get("balance") is not None:
        balances = [payload["balance"]]
    for balance in balances or []:
        if balance.get("denom") == denom:
            print(balance["amount"])
            return 0
    print("0")
    return 0


def assert_wrapper_query(path: str) -> int:
    payload = read_json(path)
    if payload.get("indexed_height") != 12:
        raise SystemExit(f"unexpected indexed height: {payload.get('indexed_height')!r}")
    actions = payload.get("actions") or []
    if payload.get("action_count") != 1 or len(actions) != 1:
        raise SystemExit("expected exactly one wrapper action")
    action = actions[0]
    expected = {
        "block_height": 11,
        "x_coordinate": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAE=",
        "is_odd": True,
        "action_type": 1,
        "amount": 42,
    }
    for key, value in expected.items():
        if action.get(key) != value:
            raise SystemExit(
                f"unexpected wrapper action {key}: {action.get(key)!r}, want {value!r}"
            )
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Archive-wrapper E2E helpers")
    subparsers = parser.add_subparsers(dest="command", required=True)

    postgres = subparsers.add_parser("render-postgres-compose")
    postgres.add_argument("--output", required=True)
    postgres.add_argument("--schema", required=True)
    postgres.add_argument("--seed", required=True)
    postgres.add_argument("--network-key")

    wrapper = subparsers.add_parser("render-wrapper-config")
    wrapper.add_argument("--output", required=True)
    wrapper.add_argument("--network-id", required=True)

    value = subparsers.add_parser("json-value")
    value.add_argument("--input", required=True)
    value.add_argument("--path", required=True)
    value.add_argument("--default")

    balance = subparsers.add_parser("bank-balance")
    balance.add_argument("--input", required=True)
    balance.add_argument("--denom", required=True)

    query = subparsers.add_parser("assert-wrapper-query")
    query.add_argument("--input", required=True)
    return parser


def main() -> int:
    args = build_parser().parse_args()
    if args.command == "render-postgres-compose":
        return render_postgres_compose(
            args.output, args.schema, args.seed, args.network_key
        )
    if args.command == "render-wrapper-config":
        return render_wrapper_config(args.output, args.network_id)
    if args.command == "json-value":
        return json_value(args.input, args.path, args.default)
    if args.command == "bank-balance":
        return bank_balance(args.input, args.denom)
    if args.command == "assert-wrapper-query":
        return assert_wrapper_query(args.input)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
