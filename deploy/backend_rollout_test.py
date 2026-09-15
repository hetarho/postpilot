"""Run the real rollout with Compose interpolation and simulated containers/HTTP."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
PREVIOUS = "1" * 40
TARGET = "2" * 40

# Only container operations and HTTP are simulated. Compose resolves the actual
# production definition, so an exported IMAGE_TAG overriding .env is observable.
COMMAND = r'''#!/usr/bin/env python3
import json, os, pathlib, subprocess, sys
args = sys.argv[1:]
root = pathlib.Path.cwd()
mode = os.environ["ROLLOUT_TEST_MODE"]
kind = pathlib.Path(sys.argv[0]).name
if kind == "docker" and "config" in args:
    os.execv(os.environ["ROLLOUT_REAL_DOCKER"], ["docker", *args])
if kind == "docker" and args[0] == "compose":
    image = subprocess.check_output(
        [os.environ["ROLLOUT_REAL_DOCKER"], "compose", "-f", "docker-compose.prod.yml", "config", "--images"],
        text=True).strip()
else:
    image = (root / "running").read_text()
with (root / "events").open("a") as log:
    log.write(json.dumps([kind, args, image]) + "\n")
if kind == "curl":
    if mode == "rollback_failure" or (mode == "health_failure" and image.endswith(os.environ["ROLLOUT_TARGET"])):
        sys.exit(22)
elif "pull" in args and mode == "pull_failure":
    sys.exit(1)
elif "up" in args:
    (root / "running").write_text(image)
    if mode == "start_failure" and "--remove-orphans" in args:
        sys.exit(1)
elif "logs" in args:
    print("migration failed: clip finalized", file=sys.stderr)
'''


class RolloutTest(unittest.TestCase):
    def run_rollout(self, mode):
        with tempfile.TemporaryDirectory() as directory:
            stack = Path(directory)
            shutil.copyfile(ROOT / "docker-compose.prod.yml", stack / "docker-compose.prod.yml")
            original = (
                f"IMAGE_TAG={PREVIOUS}\n"
                "API_UPSTREAM=postpilot-api-test\n"
                "CORS_ORIGIN=https://web.invalid\n"
                "# Preserve other settings, quotes and comments exactly.\n"
                "TEST_SETTING='literal value # unchanged'\n"
            )
            (stack / ".env").write_text(original)
            (stack / "running").write_text(f"ghcr.io/hetarho/postpilot-api:{PREVIOUS}")
            commands = stack / "bin"
            commands.mkdir()
            for name in ("docker", "curl"):
                command = commands / name
                command.write_text(COMMAND)
                command.chmod(0o755)
            environment = dict(os.environ)
            environment.update(
                PATH=str(commands) + os.pathsep + os.environ["PATH"],
                IMAGE_TAG=TARGET,
                API_ORIGIN="https://api.invalid",
                ROLLOUT_TEST_MODE=mode,
                ROLLOUT_REAL_DOCKER=shutil.which("docker") or "docker",
                ROLLOUT_TARGET=TARGET,
            )
            result = subprocess.run(
                ["sh", str(ROOT / "deploy/backend-rollout.sh")],
                cwd=stack, env=environment, capture_output=True, text=True, timeout=30,
            )
            events = [json.loads(line) for line in (stack / "events").read_text().splitlines()]
            final_env = (stack / ".env").read_text()
            self.assertFalse((stack / ".env.deploy-backup").exists())
            self.assertFalse((stack / ".env.deploy-backup.next").exists())
            self.assertEqual(final_env, original.replace(PREVIOUS, TARGET) if mode == "success" else original)
            return result, events

    def test_success_selects_incoming_sha(self):
        result, events = self.run_rollout("success")
        self.assertEqual(result.returncode, 0, result.stderr)
        updates = [event for event in events if "up" in event[1]]
        self.assertEqual([event[2].split(":")[-1] for event in updates], [TARGET])
        self.assertTrue(any("prune" in event[1] for event in events))

    def test_pull_failure_preserves_running_container(self):
        result, events = self.run_rollout("pull_failure")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any("up" in event[1] for event in events))

    def assert_rollback(self, mode, healthy):
        result, events = self.run_rollout(mode)
        self.assertNotEqual(result.returncode, 0)
        updates = [event for event in events if "up" in event[1]]
        self.assertEqual([event[2].split(":")[-1] for event in updates], [TARGET, PREVIOUS])
        probes = [event for event in events if event[0] == "curl"]
        self.assertTrue(probes[-1][2].endswith(PREVIOUS), "rollback must pass its own health gate")
        log_at = next(i for i, event in enumerate(events) if "logs" in event[1])
        self.assertLess(log_at, events.index(updates[-1]), "capture failure before replacing the container")
        self.assertIn("migration failed: clip finalized", result.stderr)
        self.assertIn("rollback healthy:" if healthy else "rollback failed;", result.stderr)
        self.assertFalse(any("prune" in event[1] for event in events))

    def test_health_failure_restores_previous_image(self):
        self.assert_rollback("health_failure", healthy=True)

    def test_start_failure_restores_previous_image(self):
        self.assert_rollback("start_failure", healthy=True)

    def test_failed_rollback_is_reported(self):
        self.assert_rollback("rollback_failure", healthy=False)


if __name__ == "__main__":
    unittest.main()
