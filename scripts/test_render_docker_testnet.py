#!/usr/bin/env python3

import argparse
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import render_docker_testnet as renderer


SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parent
RENDERER = SCRIPT_DIR / "render_docker_testnet.py"
DOCKER_TESTNET = SCRIPT_DIR / "docker_testnet.sh"


class RenderDockerTestnetTest(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp_dir.cleanup)
        self.root = Path(self.temp_dir.name)

    def args(self, mode, validator_count=3, **overrides):
        values = {
            "repo_root": str(REPO_ROOT),
            "output": str(self.root / "compose.json"),
            "generated_dir": str(self.root / "wrapper-configs"),
            "validator_count": validator_count,
            "mode": mode,
            "mina_network_id": "testnet",
            "wrapper_image": "archive-wrapper:test",
            "external_address": "external-wrapper:9095",
            "external_transport_mode": "trusted-network",
            "external_network": None,
        }
        values.update(overrides)
        return argparse.Namespace(**values)

    def test_shared_topology_has_one_wrapper_and_one_data_volume(self):
        compose = renderer.render_compose(self.args("shared"))

        wrappers = [name for name in compose["services"] if name.startswith("archive-wrapper")]
        wrapper_volumes = [name for name in compose["volumes"] if name.startswith("archive-wrapper")]
        self.assertEqual(["archive-wrapper"], wrappers)
        self.assertEqual(["archive-wrapper_data"], wrapper_volumes)
        self.assertEqual("archive-wrapper:test", compose["services"]["archive-wrapper"]["image"])
        self.assertEqual(
            "${POSTGRES_URI:?POSTGRES_URI is required}",
            compose["services"]["archive-wrapper"]["environment"]["POSTGRES_URI"],
        )

        for index in range(1, 4):
            self.assertEqual(
                "archive-wrapper:9095",
                compose["services"]["setup"]["environment"][
                    f"NODE{index}_WRAPPER_GRPC_ADDRESS"
                ],
            )
            self.assertEqual(
                {"condition": "service_healthy"},
                compose["services"][f"validator{index}"]["depends_on"][
                    "archive-wrapper"
                ],
            )

    def test_per_validator_topology_is_bijective(self):
        compose = renderer.render_compose(self.args("per-validator"))

        for index in range(1, 4):
            service_name = f"archive-wrapper-validator{index}"
            data_volume = f"{service_name}_data"
            wrapper = compose["services"][service_name]
            self.assertIn(
                f"validator{index}_data:/var/lib/pulsar:ro", wrapper["volumes"]
            )
            self.assertIn(f"{data_volume}:/var/lib/archive-wrapper", wrapper["volumes"])
            self.assertIn(data_volume, compose["volumes"])
            self.assertEqual(
                {"condition": "service_healthy"},
                compose["services"][f"validator{index}"]["depends_on"][service_name],
            )

        data_mounts = [
            mount
            for name, service in compose["services"].items()
            if name.startswith("archive-wrapper-validator")
            for mount in service["volumes"]
            if mount.endswith(":/var/lib/archive-wrapper")
        ]
        self.assertEqual(len(data_mounts), len(set(data_mounts)))

    def test_external_topology_has_no_wrapper_artifacts(self):
        compose = renderer.render_compose(
            self.args("external", external_network="wrapper-network")
        )

        self.assertFalse(
            any(name.startswith("archive-wrapper") for name in compose["services"])
        )
        self.assertFalse(
            any(name.startswith("archive-wrapper") for name in compose["volumes"])
        )
        self.assertEqual(
            {"external": True, "name": "wrapper-network"},
            compose["networks"]["archive-wrapper-external"],
        )
        for index in range(1, 4):
            validator = compose["services"][f"validator{index}"]
            self.assertEqual(
                {"setup": {"condition": "service_completed_successfully"}},
                validator["depends_on"],
            )
            self.assertEqual(["archive-wrapper-external"], validator["networks"])

    def test_one_two_and_three_validator_renders(self):
        for count in (1, 2, 3):
            with self.subTest(count=count):
                compose = renderer.render_compose(
                    self.args("per-validator", validator_count=count)
                )
                validators = [
                    name for name in compose["services"] if name.startswith("validator")
                ]
                wrappers = [
                    name
                    for name in compose["services"]
                    if name.startswith("archive-wrapper-validator")
                ]
                self.assertEqual(count, len(validators))
                self.assertEqual(count, len(wrappers))

    def test_render_is_deterministic_and_writes_expected_configs(self):
        args = self.args("per-validator")
        first = renderer.render_compose(args)
        renderer.write_wrapper_configs(args)
        first_configs = {
            path.name: path.read_text(encoding="utf-8")
            for path in Path(args.generated_dir).iterdir()
        }

        second = renderer.render_compose(args)
        renderer.write_wrapper_configs(args)
        second_configs = {
            path.name: path.read_text(encoding="utf-8")
            for path in Path(args.generated_dir).iterdir()
        }

        self.assertEqual(first, second)
        self.assertEqual(first_configs, second_configs)
        self.assertEqual(3, len(first_configs))
        for content in first_configs.values():
            config = json.loads(content)
            self.assertEqual("testnet", config["deployment_metadata"]["mina_network_id"])

    def test_external_endpoint_validation_matches_transport_policy(self):
        accepted = (
            ("127.0.0.1:9095", "loopback"),
            ("[::1]:9095", "loopback"),
            ("archive-wrapper:9095", "trusted-network"),
            ("10.0.0.2:9095", "trusted-network"),
            ("[fd00::2]:9095", "trusted-network"),
        )
        rejected = (
            ("archive-wrapper:9095", "loopback"),
            ("0.0.0.0:9095", "trusted-network"),
            ("8.8.8.8:9095", "trusted-network"),
            ("192.0.2.1:9095", "trusted-network"),
            ("http://archive-wrapper:9095", "trusted-network"),
            ("archive_wrapper:9095", "trusted-network"),
            ("127.0.0.1:0", "loopback"),
            ("127.0.0.1:9095", "invalid"),
        )

        for address, mode in accepted:
            with self.subTest(address=address, mode=mode):
                renderer.validate_wrapper_endpoint(address, mode)
        for address, mode in rejected:
            with self.subTest(address=address, mode=mode):
                with self.assertRaises(ValueError):
                    renderer.validate_wrapper_endpoint(address, mode)


class DockerTestnetScriptTest(unittest.TestCase):
    def run_script(self, command, count="2", env=None):
        process_env = os.environ.copy()
        for key in (
            "ARCHIVE_WRAPPER_MODE",
            "ARCHIVE_WRAPPER_IMAGE",
            "POSTGRES_URI",
            "ARCHIVE_WRAPPER_EXTERNAL_ADDRESS",
            "ARCHIVE_WRAPPER_EXTERNAL_TRANSPORT_MODE",
            "ARCHIVE_WRAPPER_EXTERNAL_NETWORK",
        ):
            process_env.pop(key, None)
        process_env.update(env or {})
        return subprocess.run(
            ["bash", str(DOCKER_TESTNET), command, count],
            cwd=REPO_ROOT,
            env=process_env,
            text=True,
            capture_output=True,
            check=False,
        )

    def test_config_rejects_missing_and_unknown_modes(self):
        missing = self.run_script("config", env={"ARCHIVE_WRAPPER_MODE": ""})
        self.assertNotEqual(0, missing.returncode)
        self.assertIn("ARCHIVE_WRAPPER_MODE is required", missing.stderr)

        unknown = self.run_script("config", env={"ARCHIVE_WRAPPER_MODE": "unknown"})
        self.assertNotEqual(0, unknown.returncode)
        self.assertIn("invalid ARCHIVE_WRAPPER_MODE", unknown.stderr)

    def test_config_requires_mode_specific_inputs(self):
        shared = self.run_script("config", env={"ARCHIVE_WRAPPER_MODE": "shared"})
        self.assertNotEqual(0, shared.returncode)
        self.assertIn("ARCHIVE_WRAPPER_IMAGE is required", shared.stderr)

        external = self.run_script("config", env={"ARCHIVE_WRAPPER_MODE": "external"})
        self.assertNotEqual(0, external.returncode)
        self.assertIn("ARCHIVE_WRAPPER_EXTERNAL_ADDRESS is required", external.stderr)

    def test_down_and_reset_reuse_existing_compose_without_mode_or_secret(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            compose = root / "compose.json"
            generated = root / "generated"
            compose.write_text('{"services": {}}\n', encoding="utf-8")
            generated.mkdir()

            fake_bin = root / "bin"
            fake_bin.mkdir()
            docker = fake_bin / "docker"
            docker.write_text("#!/usr/bin/env bash\nexit 0\n", encoding="utf-8")
            docker.chmod(0o755)

            env = {
                "PATH": f"{fake_bin}:{os.environ['PATH']}",
                "COMPOSE_FILE": str(compose),
                "GENERATED_DIR": str(generated),
                "PULSAR_DOCKER_PROJECT": "lifecycle-test",
            }
            down = self.run_script("down", env=env)
            self.assertEqual(0, down.returncode, down.stderr)
            self.assertTrue(compose.exists())
            self.assertTrue(generated.exists())

            reset = self.run_script("reset", env=env)
            self.assertEqual(0, reset.returncode, reset.stderr)
            self.assertFalse(compose.exists())
            self.assertFalse(generated.exists())


if __name__ == "__main__":
    unittest.main()
