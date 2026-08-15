#!/usr/bin/env python3

import json
import os
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path


SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parent
DEPLOY_SCRIPT = SCRIPT_DIR / "deploy_local_lightnet.sh"
DEFAULT_LIGHTNET_IMAGE = (
    "o1labs/mina-local-network@"
    "sha256:33e349241f5f3e8d336e5de9b35de2d4339fd8713b309e2b1b5fc375c2605b58"
)


FAKE_DOCKER = r"""#!/usr/bin/env python3
import json
import os
import sys
from pathlib import Path


args = sys.argv[1:]
with Path(os.environ["FAKE_DOCKER_LOG"]).open("a", encoding="utf-8") as log:
    log.write(json.dumps(args) + "\n")

state_path = Path(os.environ["FAKE_DOCKER_STATE"])
container_name = os.environ.get("LIGHTNET_CONTAINER", "mina-local-lightnet")


def read_state():
    if not state_path.exists():
        return "absent"
    return state_path.read_text(encoding="utf-8").strip()


def write_state(value):
    state_path.write_text(value, encoding="utf-8")


if args[:2] in (["compose", "version"], ["buildx", "version"]):
    raise SystemExit(0)

if args and args[0] == "compose":
    if "down" in args and os.environ.get("FAKE_COMPOSE_DOWN_FAIL") == "1":
        print("fake Compose cleanup failure", file=sys.stderr)
        raise SystemExit(1)
    raise SystemExit(0)

if args[:2] == ["buildx", "build"]:
    raise SystemExit(0)

if args and args[0] == "inspect":
    template = args[args.index("-f") + 1]
    reference = args[-1]
    state = read_state()
    if state == "absent" or reference not in (container_name, "lightnet-id"):
        raise SystemExit(1)
    if template == "{{.Name}}":
        print(f"/{container_name}")
    elif template == "{{.Id}}":
        print("lightnet-id")
    elif template == "{{.State.Running}}":
        print("true" if state.startswith("running-") else "false")
    elif ".Config.Labels" in template:
        if state in ("running-owned", "stopped-owned"):
            print("mina-lightnet")
        else:
            print("<no value>")
    else:
        print(f"unsupported inspect template: {template}", file=sys.stderr)
        raise SystemExit(2)
    raise SystemExit(0)

if args and args[0] == "run":
    write_state("running-owned")
    print("lightnet-id")
    raise SystemExit(0)

if args and args[0] == "rm":
    if args[-1] != "lightnet-id":
        print("refusing unexpected container removal", file=sys.stderr)
        raise SystemExit(2)
    write_state("absent")
    raise SystemExit(0)

if args and args[0] == "exec":
    index = 1
    while index < len(args) and args[index].startswith("-"):
        index += 1
    command = args[index + 1 :]
    if command[:1] == ["pg_isready"]:
        raise SystemExit(0)
    if command[:1] == ["printenv"]:
        values = {
            "POSTGRES_USER": "postgres",
            "POSTGRES_PASSWORD": "postgres",
            "POSTGRES_DB": "archive",
        }
        print(values[command[1]])
        raise SystemExit(0)
    if command[:1] == ["psql"]:
        query = " ".join(command)
        if "to_regclass" in query:
            print("t")
        elif "MAX(height)" in query:
            print("4")
        elif "SELECT EXISTS" in query:
            print("t")
        raise SystemExit(0)

if args and args[0] == "port":
    print("0.0.0.0:15432")
    raise SystemExit(0)

if args and args[0] == "logs":
    raise SystemExit(0)

print(f"unsupported fake docker invocation: {args}", file=sys.stderr)
raise SystemExit(2)
"""


FAKE_GIT = r"""#!/usr/bin/env python3
import json
import os
import sys
from pathlib import Path


args = sys.argv[1:]
with Path(os.environ["FAKE_GIT_LOG"]).open("a", encoding="utf-8") as log:
    log.write(json.dumps(args) + "\n")

if "cat-file" in args:
    raise SystemExit(0)
if "archive" in args:
    sys.stdout.buffer.write(b"fake archive")
    raise SystemExit(0)

print(f"unsupported fake git invocation: {args}", file=sys.stderr)
raise SystemExit(2)
"""


class DeployLocalLightnetScriptTest(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp_dir.cleanup)
        self.root = Path(self.temp_dir.name)
        self.home = self.root / "home"
        self.state_root = self.root / "state"
        self.wrapper_source = self.root / "archive-wrapper"
        self.fake_bin = self.root / "bin"
        self.docker_log = self.root / "docker.log"
        self.git_log = self.root / "git.log"
        self.docker_state = self.root / "docker-state"
        self.project = "selected-project"

        self.home.mkdir()
        self.state_root.mkdir()
        (self.wrapper_source / ".git").mkdir(parents=True)
        self.fake_bin.mkdir()
        self.write_executable("docker", FAKE_DOCKER)
        self.write_executable("git", FAKE_GIT)
        self.write_executable("curl", "#!/usr/bin/env bash\nexit 0\n")

        self.env = os.environ.copy()
        for key in (
            "ARCHIVE_WRAPPER_IMAGE",
            "ARCHIVE_WRAPPER_SHA",
            "ARCHIVE_WRAPPER_SOURCE",
            "BRIDGE_CONFIRMATION_DEPTH",
            "BRIDGE_MAX_BLOCK_RANGE",
            "BRIDGE_START_BLOCK_HEIGHT",
            "DOCKER_PLATFORM",
            "LIGHTNET_CONTAINER",
            "LIGHTNET_IMAGE",
            "LIGHTNET_POSTGRES_PORT",
            "LIGHTNET_READY_HEIGHT",
            "PULSAR_DOCKER_IMAGE",
            "PULSAR_DOCKER_PROJECT",
            "PULSAR_DOCKER_STATE_ROOT",
            "VALIDATOR_STARTUP_TIMEOUT",
        ):
            self.env.pop(key, None)
        self.env.update(
            {
                "ARCHIVE_WRAPPER_SHA": "deadbeef",
                "ARCHIVE_WRAPPER_SOURCE": str(self.wrapper_source),
                "DOCKER_PLATFORM": "linux/amd64",
                "FAKE_DOCKER_LOG": str(self.docker_log),
                "FAKE_DOCKER_STATE": str(self.docker_state),
                "FAKE_GIT_LOG": str(self.git_log),
                "HOME": str(self.home),
                "LIGHTNET_CONTAINER": "mina-local-lightnet",
                "LIGHTNET_READY_HEIGHT": "1",
                "PATH": f"{self.fake_bin}{os.pathsep}{self.env['PATH']}",
                "PULSAR_DOCKER_PROJECT": self.project,
                "PULSAR_DOCKER_STATE_ROOT": str(self.state_root),
                "VALIDATOR_STARTUP_TIMEOUT": "1",
            }
        )

    def write_executable(self, name, content):
        path = self.fake_bin / name
        path.write_text(textwrap.dedent(content), encoding="utf-8")
        path.chmod(0o755)

    def create_owned_project(self, project):
        generated_root = self.state_root / project
        generated_root.mkdir(parents=True)
        (generated_root / "compose.json").write_text(
            '{"services": {}}\n', encoding="utf-8"
        )
        (generated_root / ".pulsar-docker-project").write_text(
            f"pulsar-docker-project:{project}\n", encoding="utf-8"
        )
        sentinel = generated_root / "old-state"
        sentinel.write_text("keep", encoding="utf-8")
        return generated_root, sentinel

    def set_lightnet_state(self, state):
        self.docker_state.write_text(state, encoding="utf-8")

    def read_calls(self, path):
        if not path.exists():
            return []
        return [
            json.loads(line)
            for line in path.read_text(encoding="utf-8").splitlines()
        ]

    def run_deploy(self, validator_count="2", **env_overrides):
        env = self.env.copy()
        env.update(env_overrides)
        return subprocess.run(
            ["bash", str(DEPLOY_SCRIPT), validator_count],
            cwd=REPO_ROOT,
            env=env,
            text=True,
            capture_output=True,
            check=False,
            timeout=20,
        )

    def test_help_documents_cleanup_options_and_lifecycle_guide(self):
        result = self.run_deploy("--help")

        self.assertEqual(0, result.returncode, result.stderr)
        for expected in (
            "Destructive behavior:",
            "LIGHTNET_CONTAINER",
            "LIGHTNET_IMAGE",
            "LIGHTNET_READY_HEIGHT",
            "PULSAR_DOCKER_STATE_ROOT",
            "PULSAR_DOCKER_IMAGE",
            "BRIDGE_CONFIRMATION_DEPTH",
            "docs/local-lightnet-deployment.md",
        ):
            with self.subTest(expected=expected):
                self.assertIn(expected, result.stdout)
        self.assertEqual([], self.read_calls(self.docker_log))
        self.assertEqual([], self.read_calls(self.git_log))

    def test_cleans_only_selected_project_and_reuses_owned_running_lightnet(self):
        _, selected_sentinel = self.create_owned_project(self.project)
        unrelated_root, unrelated_sentinel = self.create_owned_project(
            "unrelated-project"
        )
        pulsar_home = self.home / ".pulsar"
        node_home = self.home / ".pulsar-node9"
        pulsar_home.mkdir()
        node_home.mkdir()
        (pulsar_home / "data").write_text("keep", encoding="utf-8")
        (node_home / "data").write_text("keep", encoding="utf-8")
        self.set_lightnet_state("running-owned")

        result = self.run_deploy()

        self.assertEqual(0, result.returncode, result.stderr)
        self.assertFalse(selected_sentinel.exists())
        self.assertTrue(unrelated_root.exists())
        self.assertEqual("keep", unrelated_sentinel.read_text(encoding="utf-8"))
        self.assertTrue((pulsar_home / "data").exists())
        self.assertTrue((node_home / "data").exists())
        self.assertIn("Reusing running Mina Lightnet", result.stdout)

        docker_calls = self.read_calls(self.docker_log)
        down_calls = [call for call in docker_calls if "down" in call]
        self.assertEqual(1, len(down_calls))
        self.assertIn(self.project, down_calls[0])
        self.assertIn("--volumes", down_calls[0])
        self.assertIn("--remove-orphans", down_calls[0])
        self.assertNotIn("unrelated-project", " ".join(map(str, docker_calls)))
        self.assertFalse(any(call[:1] == ["run"] for call in docker_calls))
        self.assertFalse(any(call[:1] == ["rm"] for call in docker_calls))
        self.assertFalse(
            any(call[:2] in (["image", "rm"], ["volume", "rm"], ["network", "rm"])
                for call in docker_calls)
        )

        compose = json.loads(
            (self.state_root / self.project / "compose.json").read_text(
                encoding="utf-8"
            )
        )
        self.assertEqual(
            ["host.docker.internal:host-gateway"],
            compose["services"]["archive-wrapper"]["extra_hosts"],
        )

    def test_replaces_only_an_owned_stopped_lightnet(self):
        self.set_lightnet_state("stopped-owned")

        result = self.run_deploy()

        self.assertEqual(0, result.returncode, result.stderr)
        docker_calls = self.read_calls(self.docker_log)
        self.assertIn(["rm", "-fv", "lightnet-id"], docker_calls)
        run_calls = [call for call in docker_calls if call[:1] == ["run"]]
        self.assertEqual(1, len(run_calls))
        self.assertIn("mina-local-lightnet", run_calls[0])
        self.assertEqual(DEFAULT_LIGHTNET_IMAGE, run_calls[0][-1])

    def test_allows_an_explicit_lightnet_image_override(self):
        self.set_lightnet_state("absent")
        override = "example.test/mina-lightnet@sha256:override"

        result = self.run_deploy(LIGHTNET_IMAGE=override)

        self.assertEqual(0, result.returncode, result.stderr)
        docker_calls = self.read_calls(self.docker_log)
        run_calls = [call for call in docker_calls if call[:1] == ["run"]]
        self.assertEqual(1, len(run_calls))
        self.assertEqual(override, run_calls[0][-1])

    def test_rejects_unowned_lightnet_without_mutating_project(self):
        _, selected_sentinel = self.create_owned_project(self.project)
        self.set_lightnet_state("running-unowned")

        result = self.run_deploy()

        self.assertNotEqual(0, result.returncode)
        self.assertIn("unowned container", result.stderr)
        self.assertTrue(selected_sentinel.exists())
        docker_calls = self.read_calls(self.docker_log)
        self.assertFalse(any("down" in call for call in docker_calls))
        self.assertFalse(any(call[:1] in (["rm"], ["run"]) for call in docker_calls))

    def test_invalid_validator_count_is_rejected_before_mutation(self):
        _, selected_sentinel = self.create_owned_project(self.project)
        self.set_lightnet_state("running-owned")

        for validator_count in ("0", "not-a-number"):
            with self.subTest(validator_count=validator_count):
                result = self.run_deploy(validator_count)
                self.assertNotEqual(0, result.returncode)
                self.assertIn("positive integer", result.stderr)

        self.assertTrue(selected_sentinel.exists())
        self.assertEqual([], self.read_calls(self.docker_log))
        self.assertEqual([], self.read_calls(self.git_log))

    def test_invalid_runtime_overrides_are_rejected_before_cleanup(self):
        _, selected_sentinel = self.create_owned_project(self.project)
        self.set_lightnet_state("running-owned")
        invalid_overrides = (
            ("VALIDATOR_STARTUP_TIMEOUT", "invalid", "positive integer"),
            ("BRIDGE_CONFIRMATION_DEPTH", "0", "positive integer"),
            ("BRIDGE_START_BLOCK_HEIGHT", "0", "positive integer"),
            ("BRIDGE_MAX_BLOCK_RANGE", "0", "positive integer"),
            ("DOCKER_PLATFORM", "linux/s390x", "Docker platform"),
        )

        for name, value, expected_error in invalid_overrides:
            with self.subTest(name=name, value=value):
                result = self.run_deploy(**{name: value})
                self.assertNotEqual(0, result.returncode)
                self.assertIn(expected_error, result.stderr)
                self.assertTrue(selected_sentinel.exists())
                self.assertFalse(
                    any(
                        "down" in call
                        for call in self.read_calls(self.docker_log)
                    )
                )

        self.assertEqual([], self.read_calls(self.docker_log))
        self.assertEqual([], self.read_calls(self.git_log))

    def test_compose_cleanup_failure_preserves_owned_project_metadata(self):
        generated_root, selected_sentinel = self.create_owned_project(self.project)
        self.set_lightnet_state("running-owned")

        result = self.run_deploy(FAKE_COMPOSE_DOWN_FAIL="1")

        self.assertNotEqual(0, result.returncode)
        self.assertIn("preserving project metadata", result.stderr)
        self.assertTrue(generated_root.exists())
        self.assertTrue(selected_sentinel.exists())
        self.assertTrue((generated_root / "compose.json").exists())
        self.assertTrue((generated_root / ".pulsar-docker-project").exists())
        docker_calls = self.read_calls(self.docker_log)
        self.assertTrue(any("down" in call for call in docker_calls))
        self.assertFalse(any(call[:1] in (["rm"], ["run"]) for call in docker_calls))


if __name__ == "__main__":
    unittest.main()
