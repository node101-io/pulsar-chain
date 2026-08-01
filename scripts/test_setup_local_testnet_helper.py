#!/usr/bin/env python3

import json
import subprocess
import sys
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


if __name__ == "__main__":
    unittest.main()
