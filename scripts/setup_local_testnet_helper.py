#!/usr/bin/env python3

import argparse
import base64
import hashlib
import json
import re
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional


MINA_SCALAR_FIELD = int(
    "40000000000000000000000000000000224698fc0994a8dd8c46eb2100000001", 16
)
RFC3339_TIMESTAMP_PATTERN = re.compile(
    r"^(?P<date>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})"
    r"(?:\.(?P<fraction>\d+))?"
    r"(?P<timezone>Z|[+-]\d{2}:\d{2})$"
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


def read_first_match(content: str, patterns: list[str], error_message: str) -> str:
    for pattern in patterns:
        match = re.search(pattern, content, re.MULTILINE)
        if match:
            return match.group(1).strip()

    raise SystemExit(error_message)


def read_mina_priv_key(config_path: str) -> int:
    content = read_text(config_path)
    print(
        read_first_match(
            content,
            [
                r'vote_extension:\s*\n\s*priv_key:\s*"([^"]+)"',
                r"\[vote_extension\]\s*\npriv_key\s*=\s*\"([^\"]+)\"",
            ],
            f"could not find vote extension private key in {config_path}",
        )
    )
    return 0


def read_validator_app_value(
    config_path: str, validator_index: int, section_name: str, key_name: str
) -> int:
    lines = read_text(config_path).splitlines()
    validators_start = None
    validators_indent = -1

    for index, line in enumerate(lines):
        match = re.match(r"^(\s*)validators:\s*$", line)
        if match:
            validators_start = index + 1
            validators_indent = len(match.group(1))
            break

    if validators_start is None:
        print("")
        return 0

    item_starts = []
    item_indent = None
    for index in range(validators_start, len(lines)):
        line = lines[index]
        if not line.strip():
            continue

        indent = len(line) - len(line.lstrip(" "))
        if indent <= validators_indent and not line.lstrip().startswith("-"):
            break

        if re.match(r"^\s*-\s+", line):
            if item_indent is None:
                item_indent = indent
            if indent == item_indent:
                item_starts.append(index)

    if validator_index < 1 or validator_index > len(item_starts):
        print("")
        return 0

    item_start = item_starts[validator_index - 1] + 1
    item_end = (
        item_starts[validator_index]
        if validator_index < len(item_starts)
        else len(lines)
    )
    parent_indent = item_indent if item_indent is not None else validators_indent

    app_block = find_named_block(lines, item_start, item_end, parent_indent, "app")
    if app_block is None:
        print("")
        return 0

    app_start, app_end, app_indent = app_block
    section_block = find_named_block(
        lines, app_start, app_end, app_indent, section_name
    )
    if section_block is None:
        print("")
        return 0

    section_start, section_end, section_indent = section_block
    value = read_scalar_in_block(
        lines, section_start, section_end, section_indent, key_name
    )
    print(value or "")
    return 0


def read_app_toml_string(app_path: str, table_name: str, key_name: str) -> int:
    content = read_text(app_path)
    table_pattern = re.compile(
        rf"^\[{re.escape(table_name)}\]\s*$"
        rf"(?P<body>.*?)(?=^\[|\Z)",
        re.MULTILINE | re.DOTALL,
    )
    table_match = table_pattern.search(content)
    if table_match is None:
        print("")
        return 0

    key_match = re.search(
        rf'^\s*{re.escape(key_name)}\s*=\s*"([^"]*)"\s*$',
        table_match.group("body"),
        re.MULTILINE,
    )
    print(key_match.group(1) if key_match else "")
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


def read_config_block_scalars(config_path: str, path: list[str]) -> dict[str, object]:
    lines = read_text(config_path).splitlines()
    start_index = 0
    end_index = len(lines)
    parent_indent = -1

    for block_name in path:
        block = find_named_block(
            lines, start_index, end_index, parent_indent, block_name
        )
        if block is None:
            return {}

        start_index, end_index, parent_indent = block

    direct_indent = None
    for index in range(start_index, end_index):
        line = lines[index]
        if not line.strip() or line.lstrip().startswith("#"):
            continue

        indent = len(line) - len(line.lstrip(" "))
        if indent <= parent_indent:
            break
        if direct_indent is None or indent < direct_indent:
            direct_indent = indent

    if direct_indent is None:
        return {}

    scalar_pattern = re.compile(
        r'^\s*([A-Za-z0-9_-]+):\s*'
        r'(?:(?:"([^"]*)")|(?:\'([^\']*)\')|([^#\n]+?))'
        r"\s*(?:#.*)?$"
    )
    values = {}

    for index in range(start_index, end_index):
        line = lines[index]
        if not line.strip() or line.lstrip().startswith("#"):
            continue

        indent = len(line) - len(line.lstrip(" "))
        if indent != direct_indent:
            continue

        match = scalar_pattern.match(line)
        if match is None:
            continue

        key, double_quoted, single_quoted, unquoted = match.groups()
        if double_quoted is not None:
            value = double_quoted
        elif single_quoted is not None:
            value = single_quoted
        else:
            raw_value = unquoted.strip()
            lowered = raw_value.lower()
            if lowered == "true":
                value = True
            elif lowered == "false":
                value = False
            elif lowered in ("null", "~"):
                value = None
            elif re.fullmatch(r"[-+]?\d+", raw_value):
                value = int(raw_value)
            else:
                value = raw_value

        values[key] = value

    return values


def sync_genesis_module_params(
    genesis_path: str, config_path: str, module_name: str
) -> int:
    params = read_config_block_scalars(
        config_path,
        ["genesis", "app_state", module_name, "params"],
    )
    if not params:
        raise SystemExit(f"no genesis params found for module {module_name}")

    genesis = read_json(genesis_path)
    module_state = genesis.setdefault("app_state", {}).setdefault(module_name, {})
    module_state.setdefault("params", {}).update(params)
    write_json(genesis_path, genesis)
    return 0


def verify_genesis_module_params(
    genesis_path: str, config_path: str, module_name: str
) -> int:
    expected_params = read_config_block_scalars(
        config_path,
        ["genesis", "app_state", module_name, "params"],
    )
    if not expected_params:
        raise SystemExit(f"no genesis params found for module {module_name}")

    genesis = read_json(genesis_path)
    actual_params = (
        genesis.get("app_state", {}).get(module_name, {}).get("params", {})
    )
    mismatched_keys = [
        key
        for key, expected_value in expected_params.items()
        if actual_params.get(key) != expected_value
    ]
    if mismatched_keys:
        raise SystemExit(
            f"{module_name} genesis params do not match config: "
            + ", ".join(sorted(mismatched_keys))
        )

    return 0


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
    content = read_text(config_path)
    print(
        read_first_match(
            content,
            [
                r'mina:\s*\n\s*network_id:\s*"?([^"\n]+)"?',
                r"\[mina\]\s*\nnetwork_id\s*=\s*\"([^\"]+)\"",
            ],
            f"could not find mina network id in {config_path}",
        )
    )
    return 0


def read_min_gas_price(app_path: str) -> int:
    content = read_text(app_path)
    print(
        read_first_match(
            content,
            [r'^minimum-gas-prices\s*=\s*"([^"]*)"'],
            f"could not find minimum-gas-prices in {app_path}",
        )
    )
    return 0


def set_vote_extension_height(genesis_path: str, height: str) -> int:
    genesis = read_json(genesis_path)
    genesis["consensus"]["params"]["abci"]["vote_extensions_enable_height"] = height
    write_json(genesis_path, genesis)
    return 0


def read_vote_extension_height(genesis_path: str) -> int:
    genesis = read_json(genesis_path)
    print(genesis["consensus"]["params"]["abci"]["vote_extensions_enable_height"])
    return 0


def read_genesis_chain_id(genesis_path: str) -> int:
    genesis = read_json(genesis_path)
    print(genesis["chain_id"])
    return 0


def read_consensus_pub_key(priv_validator_key_path: str) -> int:
    priv_validator_key = read_json(priv_validator_key_path)
    print(priv_validator_key["pub_key"]["value"])
    return 0


def read_account_pub_key() -> int:
    payload = json.load(sys.stdin)
    key = payload.get("key")
    if not isinstance(key, str) or not key:
        raise SystemExit("account public key JSON is missing key")
    print(key)
    return 0


def render_e2e_seed(template_path: str, output_path: str, mina_public_key: str) -> int:
    placeholder = "__E2E_MINA_PUBLIC_KEY__"
    template = read_text(template_path)
    if template.count(placeholder) != 1:
        raise SystemExit("E2E seed template must contain exactly one Mina key placeholder")
    rendered = template.replace(placeholder, mina_public_key)
    if placeholder in rendered:
        raise SystemExit("E2E seed rendering left an unresolved placeholder")
    write_text(output_path, rendered)
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
    genesis_path: str,
    mina_pub_keys: list[str],
    cosmos_keys: Optional[list[str]] = None,
    user_mina_pub_key: Optional[str] = None,
    user_cosmos_pub_key: Optional[str] = None,
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
    if (user_mina_pub_key is None) != (user_cosmos_pub_key is None):
        raise SystemExit("user Mina and Cosmos public keys must be provided together")

    keyregistry["user_key_pairs"] = (
        [
            {
                "cosmos_key": user_cosmos_pub_key,
                "mina_key": user_mina_pub_key,
            }
        ]
        if user_mina_pub_key is not None
        else []
    )
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


def verify_validator_key_pairs(genesis_path: str, cosmos_keys: list[str]) -> int:
    genesis = read_json(genesis_path)
    key_pairs = genesis.get("app_state", {}).get("keyregistry", {}).get(
        "validator_key_pairs", []
    )

    if len(key_pairs) != len(cosmos_keys):
        raise SystemExit(
            "validator key pair count mismatch: "
            f"{len(key_pairs)} pairs for {len(cosmos_keys)} validators"
        )

    for index, (key_pair, expected_cosmos_key) in enumerate(
        zip(key_pairs, cosmos_keys), start=1
    ):
        actual_cosmos_key = key_pair.get("cosmos_key")
        actual_mina_key = key_pair.get("mina_key")

        if actual_cosmos_key != expected_cosmos_key:
            raise SystemExit(
                f"validator key pair {index} cosmos key mismatch: "
                f"got {actual_cosmos_key!r}, want {expected_cosmos_key!r}"
            )

        # TODO: Compare this value with the expected Mina public key once a
        # canonical, trusted Mina key validation implementation is available.
        if not actual_mina_key:
            raise SystemExit(f"validator key pair {index} is missing a mina_key")

    return 0


def parse_positive_int(value: str) -> int:
    if re.fullmatch(r"[0-9]+", value) is None:
        raise argparse.ArgumentTypeError("value must be a positive integer")

    parsed_value = int(value)
    if parsed_value < 1:
        raise argparse.ArgumentTypeError("value must be a positive integer")

    return parsed_value


def parse_rfc3339_timestamp(timestamp: str) -> datetime:
    if not isinstance(timestamp, str) or not timestamp:
        raise ValueError("timestamp must be a non-empty string")

    match = RFC3339_TIMESTAMP_PATTERN.fullmatch(timestamp)
    if match is None:
        raise ValueError("timestamp must use RFC3339 format with a timezone")

    fraction = (match.group("fraction") or "")[:6].ljust(6, "0")
    timezone_suffix = "+00:00" if match.group("timezone") == "Z" else match.group("timezone")
    normalized_timestamp = (
        f"{match.group('date')}.{fraction}{timezone_suffix}"
    )

    try:
        parsed_timestamp = datetime.fromisoformat(normalized_timestamp)
    except ValueError as exc:
        raise ValueError(f"invalid RFC3339 timestamp: {timestamp}") from exc

    return parsed_timestamp.astimezone(timezone.utc)


def check_validator_status(
    status_json: Optional[str],
    max_block_age_seconds: int,
    now: Optional[datetime] = None,
) -> int:
    if max_block_age_seconds < 1:
        raise SystemExit("maximum block age must be a positive integer")

    if status_json is None:
        status_json = sys.stdin.read()

    if not status_json.strip():
        raise SystemExit("validator status JSON must not be empty")

    try:
        sync_info = json.loads(status_json)["result"]["sync_info"]
        catching_up = sync_info["catching_up"]
        latest_block_height = int(sync_info["latest_block_height"])
        latest_block_time = sync_info["latest_block_time"]
    except (KeyError, TypeError, ValueError, json.JSONDecodeError) as exc:
        raise SystemExit(f"unable to parse validator status response: {exc}")

    if catching_up:
        raise SystemExit("validator is still catching up")

    if latest_block_height <= 0:
        raise SystemExit(
            f"validator has not produced a positive block height yet: {latest_block_height}"
        )

    try:
        latest_block_datetime = parse_rfc3339_timestamp(latest_block_time)
    except ValueError as exc:
        raise SystemExit(f"unable to parse validator latest block time: {exc}")

    current_time = now or datetime.now(timezone.utc)
    if current_time.tzinfo is None or current_time.utcoffset() is None:
        raise SystemExit("current time must include a timezone")

    current_time = current_time.astimezone(timezone.utc)
    block_age_seconds = max(
        0.0,
        (current_time - latest_block_datetime).total_seconds(),
    )

    if block_age_seconds > max_block_age_seconds:
        raise SystemExit(
            "validator latest block is stale: "
            f"height={latest_block_height}, "
            f"latest_block_time={latest_block_time}, "
            f"age_seconds={block_age_seconds:.6f}, "
            f"max_age_seconds={max_block_age_seconds}"
        )

    return 0


def patch_bridge_genesis(
    genesis_path: str,
    confirmation_depth: str,
    contract_address: str,
    start_block_height: str,
    max_block_range: str,
    actions_reduced_root_snapshot_window_size: str,
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

    try:
        snapshot_window_size_int = int(actions_reduced_root_snapshot_window_size)
    except ValueError as exc:
        raise SystemExit(
            "invalid actions reduced root snapshot window size: "
            f"{actions_reduced_root_snapshot_window_size}"
        ) from exc

    if snapshot_window_size_int <= 0:
        raise SystemExit(
            "actions reduced root snapshot window size must be greater than 0"
        )

    genesis = read_json(genesis_path)
    app_state = genesis.setdefault("app_state", {})
    bridge = app_state.setdefault("bridge", {})
    bridge["params"] = {
        "confirmation_depth": str(confirmation_depth_int),
        "contract_address": contract_address,
        "start_block_height": str(start_block_height_int),
        "max_block_range": str(max_block_range_int),
        "actions_reduced_root_snapshot_window_size": str(snapshot_window_size_int),
    }

    initial_mina_height = str(start_block_height_int - 1)
    bridge["bridge_state"] = {
        "latest_fetched_mina_height": initial_mina_height,
        "valid_action_hashes": [],
        "valid_action_hashes_cosmos_block_height": "0",
        "start_mina_height": initial_mina_height,
    }

    bridge.setdefault(
        "actions_reduced_root_snapshots",
        [
            {
                "cosmos_block_height": "0",
                "actions_reduced_root": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
            }
        ],
    )

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
    matching_indexes = []
    for index in range(table_start + 1, table_end):
        if key_pattern.match(lines[index]):
            matching_indexes.append(index)

    if matching_indexes:
        lines[matching_indexes[0]] = key_line
        for duplicate_index in reversed(matching_indexes[1:]):
            del lines[duplicate_index]
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
    wrapper_grpc_transport_mode: str,
) -> int:
    app_toml = read_text(app_path)
    app_toml = app_toml.replace(
        'minimum-gas-prices = ""',
        f'minimum-gas-prices = "{min_gas_price}"',
        1,
    )

    app_toml = upsert_toml_key(app_toml, "vote_extension", "priv_key", mina_priv_key)

    app_toml = upsert_toml_key(app_toml, "mina", "network_id", mina_network_id)

    if not wrapper_grpc_address or wrapper_grpc_address != wrapper_grpc_address.strip():
        raise SystemExit("wrapper gRPC address must be non-empty without surrounding whitespace")
    if wrapper_grpc_transport_mode not in ("loopback", "trusted-network"):
        raise SystemExit(
            "wrapper gRPC transport mode must be loopback or trusted-network"
        )

    app_toml = upsert_toml_key(
        app_toml, "bridge", "wrapper_grpc_address", wrapper_grpc_address
    )
    app_toml = upsert_toml_key(
        app_toml,
        "bridge",
        "wrapper_grpc_transport_mode",
        wrapper_grpc_transport_mode,
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

    read_wrapper_config = subparsers.add_parser("read-validator-wrapper-config")
    read_wrapper_config.add_argument("--config", required=True)
    read_wrapper_config.add_argument("--index", required=True, type=parse_positive_int)
    read_wrapper_config.add_argument(
        "--key",
        required=True,
        choices=("wrapper_grpc_address", "wrapper_grpc_transport_mode"),
    )

    read_app_wrapper_config = subparsers.add_parser("read-app-wrapper-config")
    read_app_wrapper_config.add_argument("--app", required=True)
    read_app_wrapper_config.add_argument(
        "--key",
        required=True,
        choices=("wrapper_grpc_address", "wrapper_grpc_transport_mode"),
    )

    read_bridge_param_cmd = subparsers.add_parser("read-bridge-genesis-param")
    read_bridge_param_cmd.add_argument("--config", required=True)
    read_bridge_param_cmd.add_argument("--key", required=True)

    read_gas_price = subparsers.add_parser("read-min-gas-price")
    read_gas_price.add_argument("--app", required=True)

    read_consensus_key = subparsers.add_parser("read-consensus-pub-key")
    read_consensus_key.add_argument("--priv-validator-key", required=True)

    subparsers.add_parser("read-account-pub-key")

    render_seed = subparsers.add_parser("render-e2e-seed")
    render_seed.add_argument("--template", required=True)
    render_seed.add_argument("--output", required=True)
    render_seed.add_argument("--mina-public-key", required=True)

    set_height = subparsers.add_parser("set-vote-extension-height")
    set_height.add_argument("--genesis", required=True)
    set_height.add_argument("--height", required=True)

    read_height = subparsers.add_parser("read-vote-extension-height")
    read_height.add_argument("--genesis", required=True)

    read_chain_id = subparsers.add_parser("read-genesis-chain-id")
    read_chain_id.add_argument("--genesis", required=True)

    generate_mina_key = subparsers.add_parser("generate-default-mina-priv-key")
    generate_mina_key.add_argument("--index", required=True)

    validate_mina_key = subparsers.add_parser("validate-mina-priv-key")
    validate_mina_key.add_argument("--index", required=True)
    validate_mina_key.add_argument("--mina-priv-key", required=True)

    patch_registry = subparsers.add_parser("patch-keyregistry")
    patch_registry.add_argument("--genesis", required=True)
    patch_registry.add_argument("--cosmos-key", action="append")
    patch_registry.add_argument("--mina-pub-key", action="append", required=True)
    patch_registry.add_argument("--user-mina-pub-key")
    patch_registry.add_argument("--user-cosmos-pub-key")

    patch_bridge = subparsers.add_parser("patch-bridge-genesis")
    patch_bridge.add_argument("--genesis", required=True)
    patch_bridge.add_argument("--confirmation-depth", required=True)
    patch_bridge.add_argument("--contract-address", required=True)
    patch_bridge.add_argument("--start-block-height", required=True)
    patch_bridge.add_argument("--max-block-range", required=True)
    patch_bridge.add_argument(
        "--actions-reduced-root-snapshot-window-size", required=True
    )

    sync_params = subparsers.add_parser("sync-genesis-module-params")
    sync_params.add_argument("--genesis", required=True)
    sync_params.add_argument("--config", required=True)
    sync_params.add_argument("--module", required=True)

    verify_params = subparsers.add_parser("verify-genesis-module-params")
    verify_params.add_argument("--genesis", required=True)
    verify_params.add_argument("--config", required=True)
    verify_params.add_argument("--module", required=True)

    verify_registry = subparsers.add_parser("verify-validator-key-pairs")
    verify_registry.add_argument("--genesis", required=True)
    verify_registry.add_argument("--cosmos-key", action="append", required=True)

    check_status = subparsers.add_parser("check-validator-status")
    check_status.add_argument("--status-json")
    check_status.add_argument(
        "--max-block-age-seconds",
        required=True,
        type=parse_positive_int,
    )

    update_app = subparsers.add_parser("update-app-config")
    update_app.add_argument("--app", required=True)
    update_app.add_argument("--min-gas-price", required=True)
    update_app.add_argument("--mina-priv-key", required=True)
    update_app.add_argument("--mina-network-id", required=True)
    update_app.add_argument("--wrapper-grpc-address", required=True)
    update_app.add_argument("--wrapper-grpc-transport-mode", required=True)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    if args.command == "read-mina-priv-key":
        return read_mina_priv_key(args.config)
    if args.command == "read-validator-wrapper-config":
        return read_validator_app_value(
            args.config, args.index, "bridge", args.key
        )
    if args.command == "read-app-wrapper-config":
        return read_app_toml_string(args.app, "bridge", args.key)
    if args.command == "read-bridge-genesis-param":
        return read_bridge_genesis_param(args.config, args.key)
    if args.command == "read-mina-network-id":
        return read_mina_network_id(args.config)
    if args.command == "read-min-gas-price":
        return read_min_gas_price(args.app)
    if args.command == "read-consensus-pub-key":
        return read_consensus_pub_key(args.priv_validator_key)
    if args.command == "read-account-pub-key":
        return read_account_pub_key()
    if args.command == "render-e2e-seed":
        return render_e2e_seed(args.template, args.output, args.mina_public_key)
    if args.command == "set-vote-extension-height":
        return set_vote_extension_height(args.genesis, args.height)
    if args.command == "read-vote-extension-height":
        return read_vote_extension_height(args.genesis)
    if args.command == "read-genesis-chain-id":
        return read_genesis_chain_id(args.genesis)
    if args.command == "generate-default-mina-priv-key":
        return generate_default_mina_priv_key(args.index)
    if args.command == "validate-mina-priv-key":
        return validate_mina_priv_key(args.index, args.mina_priv_key)
    if args.command == "patch-keyregistry":
        return patch_keyregistry(
            args.genesis,
            args.mina_pub_key,
            args.cosmos_key,
            args.user_mina_pub_key,
            args.user_cosmos_pub_key,
        )
    if args.command == "patch-bridge-genesis":
        return patch_bridge_genesis(
            args.genesis,
            args.confirmation_depth,
            args.contract_address,
            args.start_block_height,
            args.max_block_range,
            args.actions_reduced_root_snapshot_window_size,
        )
    if args.command == "sync-genesis-module-params":
        return sync_genesis_module_params(
            args.genesis,
            args.config,
            args.module,
        )
    if args.command == "verify-genesis-module-params":
        return verify_genesis_module_params(
            args.genesis,
            args.config,
            args.module,
        )
    if args.command == "verify-validator-key-pairs":
        return verify_validator_key_pairs(args.genesis, args.cosmos_key)
    if args.command == "check-validator-status":
        return check_validator_status(
            args.status_json,
            args.max_block_age_seconds,
        )
    if args.command == "update-app-config":
        return update_app_config(
            args.app,
            args.min_gas_price,
            args.mina_priv_key,
            args.mina_network_id,
            args.wrapper_grpc_address,
            args.wrapper_grpc_transport_mode,
        )

    parser.print_help(sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
