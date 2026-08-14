#!/usr/bin/env python3

import argparse
import ipaddress
import json
import os
import re
from pathlib import Path


WRAPPER_MODES = ("shared", "per-validator", "external")
TRANSPORT_MODES = ("loopback", "trusted-network")
DNS_LABEL = re.compile(r"^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$")
PRIVATE_NETWORKS = tuple(
    ipaddress.ip_network(network)
    for network in ("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7")
)


def parse_positive_int(value: str) -> int:
    try:
        parsed = int(value)
    except ValueError as exc:
        raise argparse.ArgumentTypeError("must be a positive integer") from exc
    if parsed < 1:
        raise argparse.ArgumentTypeError("must be a positive integer")
    return parsed


def split_endpoint(address: str) -> tuple[str, int]:
    if not address or address != address.strip():
        raise ValueError("address must be non-empty without surrounding whitespace")
    if "://" in address or "@" in address:
        raise ValueError("URL schemes and userinfo are not allowed")

    if address.startswith("["):
        match = re.fullmatch(r"\[([^]]+)]:(\d+)", address)
        if match is None:
            raise ValueError("expected [IPv6]:port")
        host, port_text = match.groups()
    else:
        if address.count(":") != 1:
            raise ValueError("expected host:port")
        host, port_text = address.rsplit(":", 1)

    if not host or not port_text.isdigit():
        raise ValueError("host and numeric port are required")
    port = int(port_text)
    if port < 1 or port > 65535:
        raise ValueError("port must be between 1 and 65535")
    return host, port


def validate_wrapper_endpoint(address: str, transport_mode: str) -> None:
    if transport_mode not in TRANSPORT_MODES:
        raise ValueError("transport mode must be loopback or trusted-network")

    host, _ = split_endpoint(address)
    if "%" in host:
        raise ValueError("zoned IPv6 addresses are not allowed")

    try:
        ip = ipaddress.ip_address(host)
    except ValueError:
        ip = None

    if ip is not None:
        if ip.is_unspecified:
            raise ValueError("wildcard addresses are not valid client endpoints")
        if transport_mode == "loopback" and not ip.is_loopback:
            raise ValueError("loopback mode requires a loopback IP")
        if transport_mode == "trusted-network" and not (
            ip.is_loopback or any(ip in network for network in PRIVATE_NETWORKS)
        ):
            raise ValueError("trusted-network mode requires a private or loopback IP")
        return

    if transport_mode != "trusted-network":
        raise ValueError("loopback mode requires a literal loopback IP")
    if all(character.isdigit() or character == "." for character in host):
        raise ValueError("malformed IP address")
    if len(host) > 253 or host.endswith("."):
        raise ValueError("invalid DNS service name")
    if not all(DNS_LABEL.fullmatch(label) for label in host.split(".")):
        raise ValueError("invalid DNS service name")


def wrapper_config(network_id: str) -> str:
    return json.dumps(
        {
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
        },
        indent=2,
    ) + "\n"


def validator_ports(index: int) -> list[dict]:
    return [
        {
            "host_ip": "${PULSAR_BIND_HOST:-127.0.0.1}",
            "target": target,
            "published": f"${{VALIDATOR{index}_HOST_{name}_PORT:-{default}}}",
            "protocol": "tcp",
        }
        for name, target, default in (
            ("RPC", 26657, 26657 + ((index - 1) * 10)),
            ("API", 1317, 1317 + index - 1),
            ("GRPC", 9090, 9090 + index - 1),
            ("PPROF", 6060, 6060 + index - 1),
        )
    ]


def wrapper_service(
    image: str,
    config_mount: str,
    validator_volume: str,
    data_volume: str,
) -> dict:
    return {
        "image": image,
        "read_only": True,
        "restart": "unless-stopped",
        "stop_grace_period": "15s",
        "depends_on": {"setup": {"condition": "service_completed_successfully"}},
        "environment": {"POSTGRES_URI": "${POSTGRES_URI:-}"},
        "extra_hosts": ["host.docker.internal:host-gateway"],
        "volumes": [
            f"{config_mount}:/etc/archive-wrapper/config.yaml:ro",
            f"{validator_volume}:/var/lib/pulsar:ro",
            f"{data_volume}:/var/lib/archive-wrapper",
        ],
        "tmpfs": ["/run/archive-wrapper:uid=65532,gid=65532,mode=0700"],
    }


def render_compose(args: argparse.Namespace) -> dict:
    compose_path = Path(args.output).resolve()
    repo_root = Path(args.repo_root).resolve()
    generated_dir = Path(args.generated_dir).resolve()
    build_context = os.path.relpath(repo_root, compose_path.parent)
    generated_mount_dir = os.path.relpath(generated_dir, compose_path.parent)
    if not generated_mount_dir.startswith((".", "/")):
        generated_mount_dir = f"./{generated_mount_dir}"

    services = {
        "setup": {
            "image": "${PULSAR_DOCKER_IMAGE:-pulsar-chain:local}",
            "build": {"context": build_context, "dockerfile": "Dockerfile"},
            "command": ["setup-local-testnet", str(args.validator_count)],
            "restart": "no",
            "volumes": [
                f"validator{index}_data:/testnet/.pulsar-node{index}"
                for index in range(1, args.validator_count + 1)
            ],
            "environment": {},
        }
    }
    services["setup"]["environment"].update(
        {
            "CONFIRMATION_DEPTH": "${BRIDGE_CONFIRMATION_DEPTH:-}",
            "START_BLOCK_HEIGHT": "${BRIDGE_START_BLOCK_HEIGHT:-}",
            "MAX_BLOCK_RANGE": "${BRIDGE_MAX_BLOCK_RANGE:-}",
            "E2E_USER_MINA_PRIV_KEY": "${E2E_USER_MINA_PRIV_KEY:-}",
            "MIN_GAS_PRICE": "${E2E_MIN_GAS_PRICE:-}",
        }
    )
    volumes = {
        f"validator{index}_data": {}
        for index in range(1, args.validator_count + 1)
    }

    endpoint_by_validator = {}
    mode_by_validator = {}
    wrapper_dependency_by_validator = {}

    if args.mode == "shared":
        endpoint_by_validator = {
            index: "archive-wrapper:9095"
            for index in range(1, args.validator_count + 1)
        }
        mode_by_validator = {
            index: "trusted-network" for index in endpoint_by_validator
        }
        wrapper_dependency_by_validator = {
            index: "archive-wrapper" for index in endpoint_by_validator
        }
        config_name = "archive-wrapper.yaml"
        services["archive-wrapper"] = wrapper_service(
            args.wrapper_image,
            f"{generated_mount_dir}/{config_name}",
            "validator1_data",
            "archive-wrapper_data",
        )
        volumes["archive-wrapper_data"] = {}
    elif args.mode == "per-validator":
        for index in range(1, args.validator_count + 1):
            service_name = f"archive-wrapper-validator{index}"
            data_volume = f"{service_name}_data"
            config_name = f"{service_name}.yaml"
            endpoint_by_validator[index] = f"{service_name}:9095"
            mode_by_validator[index] = "trusted-network"
            wrapper_dependency_by_validator[index] = service_name
            services[service_name] = wrapper_service(
                args.wrapper_image,
                f"{generated_mount_dir}/{config_name}",
                f"validator{index}_data",
                data_volume,
            )
            volumes[data_volume] = {}
    else:
        validate_wrapper_endpoint(args.external_address, args.external_transport_mode)
        endpoint_by_validator = {
            index: args.external_address
            for index in range(1, args.validator_count + 1)
        }
        mode_by_validator = {
            index: args.external_transport_mode for index in endpoint_by_validator
        }

    for index in range(1, args.validator_count + 1):
        services["setup"]["environment"][
            f"NODE{index}_WRAPPER_GRPC_ADDRESS"
        ] = endpoint_by_validator[index]
        services["setup"]["environment"][
            f"NODE{index}_WRAPPER_GRPC_TRANSPORT_MODE"
        ] = mode_by_validator[index]

        depends_on = {"setup": {"condition": "service_completed_successfully"}}
        wrapper_dependency = wrapper_dependency_by_validator.get(index)
        if wrapper_dependency:
            depends_on[wrapper_dependency] = {"condition": "service_healthy"}

        validator = {
            "image": "${PULSAR_DOCKER_IMAGE:-pulsar-chain:local}",
            "command": ["start-validator", str(index)],
            "depends_on": depends_on,
            "restart": "unless-stopped",
            "volumes": [f"validator{index}_data:/testnet/.pulsar-node{index}"],
            "environment": {
                "PULSAR_MAX_BLOCK_AGE_SECONDS": "${PULSAR_MAX_BLOCK_AGE_SECONDS:-30}",
                "VALIDATOR_HOME": f"/testnet/.pulsar-node{index}",
            },
            "ports": validator_ports(index),
            "healthcheck": {
                "test": [
                    "CMD",
                    "/opt/pulsar/scripts/docker_entrypoint.sh",
                    "healthcheck-validator",
                ],
                "interval": "5s",
                "timeout": "8s",
                "retries": 20,
                "start_period": "20s",
            },
        }
        if args.mode == "external" and args.external_network:
            validator["networks"] = ["archive-wrapper-external"]
        services[f"validator{index}"] = validator

    compose = {"services": services, "volumes": volumes}
    if args.mode == "external" and args.external_network:
        compose["networks"] = {
            "archive-wrapper-external": {
                "external": True,
                "name": args.external_network,
            }
        }
    return compose


def write_wrapper_configs(args: argparse.Namespace) -> None:
    if args.mode == "external":
        return

    generated_dir = Path(args.generated_dir)
    generated_dir.mkdir(parents=True, exist_ok=True)
    names = (
        ["archive-wrapper.yaml"]
        if args.mode == "shared"
        else [
            f"archive-wrapper-validator{index}.yaml"
            for index in range(1, args.validator_count + 1)
        ]
    )
    content = wrapper_config(args.mina_network_id)
    for name in names:
        (generated_dir / name).write_text(content, encoding="utf-8")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Render a wrapper-aware Pulsar testnet")
    parser.add_argument("--repo-root", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--generated-dir", required=True)
    parser.add_argument("--validator-count", required=True, type=parse_positive_int)
    parser.add_argument("--mode", required=True, choices=WRAPPER_MODES)
    parser.add_argument("--mina-network-id", required=True)
    parser.add_argument("--wrapper-image")
    parser.add_argument("--external-address")
    parser.add_argument("--external-transport-mode")
    parser.add_argument("--external-network")
    return parser


def main() -> int:
    args = build_parser().parse_args()
    if args.mode in ("shared", "per-validator") and not args.wrapper_image:
        raise SystemExit("--wrapper-image is required for shared and per-validator modes")
    if args.mode == "external" and (
        not args.external_address or not args.external_transport_mode
    ):
        raise SystemExit(
            "--external-address and --external-transport-mode are required for external mode"
        )

    compose = render_compose(args)
    write_wrapper_configs(args)
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(compose, indent=2) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
