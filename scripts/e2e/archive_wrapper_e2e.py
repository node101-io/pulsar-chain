#!/usr/bin/env python3

import argparse
import json
import sys
from pathlib import Path


POSTGRES_IMAGE = "postgres:17-bookworm@sha256:4f736ae292687621d4dbe0d499ffd024a36bd2ee7d8ca6f2ccd4c800f047b394"


def render_postgres_compose(output: str, schema: str, seed: str) -> int:
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
    Path(output).write_text(json.dumps(compose, indent=2) + "\n", encoding="utf-8")
    return 0


def read_json(path: str):
    if path == "-":
        return json.load(sys.stdin)
    return json.loads(Path(path).read_text(encoding="utf-8"))


def json_value(path: str, field_path: str) -> int:
    value = read_json(path)
    for field in field_path.split("."):
        if isinstance(value, list):
            value = value[int(field)]
        else:
            value = value[field]
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
    expected = {"block_height": 11, "action_type": 1, "amount": 42}
    for key, value in expected.items():
        if action.get(key) != value:
            raise SystemExit(
                f"unexpected wrapper action {key}: {action.get(key)!r}, want {value!r}"
            )
    if not action.get("fee_payer"):
        raise SystemExit("wrapper action is missing fee_payer")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Archive-wrapper E2E helpers")
    subparsers = parser.add_subparsers(dest="command", required=True)

    postgres = subparsers.add_parser("render-postgres-compose")
    postgres.add_argument("--output", required=True)
    postgres.add_argument("--schema", required=True)
    postgres.add_argument("--seed", required=True)

    value = subparsers.add_parser("json-value")
    value.add_argument("--input", required=True)
    value.add_argument("--path", required=True)

    balance = subparsers.add_parser("bank-balance")
    balance.add_argument("--input", required=True)
    balance.add_argument("--denom", required=True)

    query = subparsers.add_parser("assert-wrapper-query")
    query.add_argument("--input", required=True)
    return parser


def main() -> int:
    args = build_parser().parse_args()
    if args.command == "render-postgres-compose":
        return render_postgres_compose(args.output, args.schema, args.seed)
    if args.command == "json-value":
        return json_value(args.input, args.path)
    if args.command == "bank-balance":
        return bank_balance(args.input, args.denom)
    if args.command == "assert-wrapper-query":
        return assert_wrapper_query(args.input)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
