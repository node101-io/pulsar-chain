#!/usr/bin/env python3

import json
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime, timezone
from pathlib import Path

import setup_local_testnet_helper as helper


NOW = datetime(2026, 8, 1, 20, 38, 46, tzinfo=timezone.utc)
HELPER_PATH = Path(__file__).with_name("setup_local_testnet_helper.py")


def validator_status(
    latest_block_time="2026-08-01T20:38:17Z",
    latest_block_height="1",
    catching_up=False,
):
    return json.dumps(
        {
            "result": {
                "sync_info": {
                    "catching_up": catching_up,
                    "latest_block_height": latest_block_height,
                    "latest_block_time": latest_block_time,
                }
            }
        }
    )


class CheckValidatorStatusTest(unittest.TestCase):
    def check_status(self, status_json, max_block_age_seconds=30):
        return helper.check_validator_status(
            status_json,
            max_block_age_seconds,
            now=NOW,
        )

    def test_accepts_fresh_nanosecond_timestamp(self):
        status_json = validator_status("2026-08-01T20:38:16.657271353Z")

        self.assertEqual(0, self.check_status(status_json))

    def test_accepts_timestamp_without_fractional_seconds(self):
        self.assertEqual(0, self.check_status(validator_status()))

    def test_accepts_positive_and_negative_timezone_offsets(self):
        timestamps = (
            "2026-08-01T23:38:16+03:00",
            "2026-08-01T16:38:16-04:00",
        )

        for timestamp in timestamps:
            with self.subTest(timestamp=timestamp):
                self.assertEqual(0, self.check_status(validator_status(timestamp)))

    def test_accepts_block_at_exact_age_limit(self):
        self.assertEqual(
            0,
            self.check_status(validator_status("2026-08-01T20:38:16Z")),
        )

    def test_rejects_block_older_than_age_limit(self):
        with self.assertRaisesRegex(
            SystemExit,
            r"height=7, .*age_seconds=30\.000001, max_age_seconds=30",
        ):
            self.check_status(
                validator_status(
                    "2026-08-01T20:38:15.999999Z",
                    latest_block_height="7",
                )
            )

    def test_accepts_future_block_time_as_zero_age(self):
        self.assertEqual(
            0,
            self.check_status(validator_status("2026-08-01T20:39:46Z")),
        )

    def test_rejects_missing_empty_and_malformed_block_time(self):
        missing_time = json.loads(validator_status())
        del missing_time["result"]["sync_info"]["latest_block_time"]

        invalid_statuses = (
            json.dumps(missing_time),
            validator_status(""),
            validator_status("2026-02-30T20:38:17Z"),
            validator_status("2026-08-01T20:38:17"),
            validator_status("not-a-timestamp"),
        )

        for status_json in invalid_statuses:
            with self.subTest(status_json=status_json):
                with self.assertRaises(SystemExit):
                    self.check_status(status_json)

    def test_rejects_non_positive_block_height(self):
        for height in ("0", "-1"):
            with self.subTest(height=height):
                with self.assertRaisesRegex(SystemExit, "positive block height"):
                    self.check_status(validator_status(latest_block_height=height))

    def test_rejects_validator_that_is_catching_up(self):
        with self.assertRaisesRegex(SystemExit, "still catching up"):
            self.check_status(validator_status(catching_up=True))

    def test_rejects_non_positive_maximum_block_age(self):
        for maximum_age in (0, -1):
            with self.subTest(maximum_age=maximum_age):
                with self.assertRaisesRegex(SystemExit, "positive integer"):
                    self.check_status(validator_status(), maximum_age)

    def test_rejects_current_time_without_timezone(self):
        with self.assertRaisesRegex(SystemExit, "must include a timezone"):
            helper.check_validator_status(
                validator_status(),
                30,
                now=datetime(2026, 8, 1, 20, 38, 46),
            )


class CheckValidatorStatusCLITest(unittest.TestCase):
    def test_rejects_invalid_maximum_block_age_arguments(self):
        for maximum_age in ("", "0", "-1", "1.5", "1_0", " 30", "not-a-number"):
            with self.subTest(maximum_age=maximum_age):
                result = subprocess.run(
                    [
                        sys.executable,
                        str(HELPER_PATH),
                        "check-validator-status",
                        "--max-block-age-seconds",
                        maximum_age,
                    ],
                    input=validator_status(),
                    text=True,
                    capture_output=True,
                    check=False,
                )

                self.assertNotEqual(0, result.returncode)
                self.assertIn("positive integer", result.stderr)


class WrapperConfigTest(unittest.TestCase):
    def write_temp(self, content):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        path = Path(temp_dir.name) / "config"
        path.write_text(content, encoding="utf-8")
        return path

    def capture_stdout(self, function, *args):
        original_stdout = sys.stdout
        with tempfile.TemporaryFile(mode="w+") as output:
            sys.stdout = output
            try:
                self.assertEqual(0, function(*args))
                output.seek(0)
                return output.read().strip()
            finally:
                sys.stdout = original_stdout

    def test_reads_node_specific_wrapper_values(self):
        config = self.write_temp(
            """validators:
- name: one
  app:
    bridge:
      wrapper_grpc_address: "archive-wrapper-one:9095"
      wrapper_grpc_transport_mode: "trusted-network"
- name: two
  app:
    bridge:
      wrapper_grpc_address: "127.0.0.1:9095"
      wrapper_grpc_transport_mode: "loopback"
"""
        )

        self.assertEqual(
            "archive-wrapper-one:9095",
            self.capture_stdout(
                helper.read_validator_app_value,
                str(config),
                1,
                "bridge",
                "wrapper_grpc_address",
            ),
        )
        self.assertEqual(
            "loopback",
            self.capture_stdout(
                helper.read_validator_app_value,
                str(config),
                2,
                "bridge",
                "wrapper_grpc_transport_mode",
            ),
        )

    def test_update_app_config_upserts_address_and_mode_without_duplicates(self):
        app_config = self.write_temp(
            """minimum-gas-prices = ""

[bridge]
wrapper_grpc_address = "old:9095"
wrapper_grpc_address = "duplicate:9095"

[vote_extension]
priv_key = "old"

[mina]
network_id = "old"
"""
        )

        self.assertEqual(
            0,
            helper.update_app_config(
                str(app_config),
                "0pmina",
                "mina-private",
                "testnet",
                "archive-wrapper:9095",
                "trusted-network",
            ),
        )

        content = app_config.read_text(encoding="utf-8")
        self.assertEqual(1, content.count("wrapper_grpc_address ="))
        self.assertEqual(1, content.count("wrapper_grpc_transport_mode ="))
        self.assertIn('wrapper_grpc_address = "archive-wrapper:9095"', content)
        self.assertIn('wrapper_grpc_transport_mode = "trusted-network"', content)

    def test_update_app_config_rejects_empty_or_invalid_wrapper_values(self):
        for address, mode in (
            ("", "loopback"),
            (" archive-wrapper:9095", "trusted-network"),
            ("archive-wrapper:9095", "invalid"),
        ):
            with self.subTest(address=address, mode=mode):
                app_config = self.write_temp(
                    'minimum-gas-prices = ""\n[vote_extension]\n[mina]\n'
                )
                with self.assertRaises(SystemExit):
                    helper.update_app_config(
                        str(app_config),
                        "0pmina",
                        "mina-private",
                        "testnet",
                        address,
                        mode,
                    )


class E2EFixtureTest(unittest.TestCase):
    def write_temp(self, name, content):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        path = Path(temp_dir.name) / name
        path.write_text(content, encoding="utf-8")
        return path

    def test_patch_keyregistry_adds_explicit_user_pair(self):
        genesis = self.write_temp(
            "genesis.json",
            json.dumps(
                {
                    "app_state": {
                        "keyregistry": {},
                        "genutil": {"gen_txs": []},
                    }
                }
            ),
        )

        self.assertEqual(
            0,
            helper.patch_keyregistry(
                str(genesis),
                ["validator-mina"],
                ["validator-cosmos"],
                "user-mina",
                "user-cosmos",
            ),
        )
        payload = json.loads(genesis.read_text(encoding="utf-8"))
        self.assertEqual(
            [{"cosmos_key": "user-cosmos", "mina_key": "user-mina"}],
            payload["app_state"]["keyregistry"]["user_key_pairs"],
        )

    def test_patch_bridge_genesis_sets_initial_bridge_state_fields(self):
        genesis = self.write_temp("genesis.json", json.dumps({"app_state": {}}))

        self.assertEqual(
            0,
            helper.patch_bridge_genesis(
                str(genesis),
                "32",
                "B62qjRDirGFRf5dvNcGzMs5oWzQ2VyNcygnoKM2MkxB9PFUp7Utdraf",
                "10",
                "1000",
                "4",
            ),
        )

        payload = json.loads(genesis.read_text(encoding="utf-8"))
        bridge = payload["app_state"]["bridge"]
        self.assertEqual(
            {
                "confirmation_depth": "32",
                "contract_address": "B62qjRDirGFRf5dvNcGzMs5oWzQ2VyNcygnoKM2MkxB9PFUp7Utdraf",
                "start_block_height": "10",
                "max_block_range": "1000",
                "actions_reduced_root_snapshot_window_size": "4",
            },
            bridge["params"],
        )
        self.assertEqual(
            {
                "latest_fetched_mina_height": "9",
                "valid_action_hashes": [],
                "valid_action_hashes_cosmos_block_height": "0",
                "start_mina_height": "9",
            },
            bridge["bridge_state"],
        )

    def test_render_e2e_seed_replaces_exactly_one_placeholder(self):
        template = self.write_temp(
            "seed.sql.tmpl", "fee_payer = '__E2E_MINA_PUBLIC_KEY__';\n"
        )
        output = template.with_name("seed.sql")

        self.assertEqual(
            0,
            helper.render_e2e_seed(str(template), str(output), "mina-public"),
        )
        self.assertEqual(
            "fee_payer = 'mina-public';\n", output.read_text(encoding="utf-8")
        )

    def test_render_e2e_seed_rejects_missing_or_duplicate_placeholders(self):
        for content in ("SELECT 1;\n", "__E2E_MINA_PUBLIC_KEY__ __E2E_MINA_PUBLIC_KEY__"):
            with self.subTest(content=content):
                template = self.write_temp("seed.sql.tmpl", content)
                with self.assertRaises(SystemExit):
                    helper.render_e2e_seed(
                        str(template), str(template.with_name("seed.sql")), "key"
                    )


if __name__ == "__main__":
    unittest.main()
