"""A queued old run must not replace a newer requested deployment."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('latest-rollout.sh')


class LatestRollout(unittest.TestCase):
    def check_run(self, latest='120', exit_code=0):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            gh = root / 'gh'
            gh.write_text('#!/bin/sh\nprintf "%s\\n" "$*" > "$CALLS"\nprintf "%s\\n" "$LATEST"\nexit "$API_EXIT"\n')
            gh.chmod(0o755)
            output, summary, calls = (root / name for name in ['output', 'summary', 'calls'])
            env = dict(os.environ, PATH=str(root)+os.pathsep+os.environ['PATH'],
                       GITHUB_REPOSITORY='owner/project', GITHUB_REF_NAME='main',
                       GITHUB_RUN_ID='120', GITHUB_OUTPUT=str(output), GITHUB_STEP_SUMMARY=str(summary),
                       CALLS=str(calls), LATEST=latest, API_EXIT=str(exit_code))
            result = subprocess.run(['sh', str(SCRIPT)], env=env, capture_output=True, text=True, timeout=5)
            read = lambda p: p.read_text() if p.exists() else ''
            return result, read(output), read(summary), read(calls)

    def test_current_run_can_deploy_even_if_branch_has_unrelated_later_commits(self):
        result, output, summary, request = self.check_run()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(output, 'deploy=true\n')
        self.assertEqual(summary, '')
        self.assertIn('actions/workflows/deploy-backend.yml/runs', request)
        self.assertIn('branch=main', request)
        self.assertNotIn('git/ref', request)

    def test_newer_run_prevents_server_mutation(self):
        result, output, summary, _ = self.check_run(latest='121')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(output, 'deploy=false\n')
        self.assertIn('will not change the server', summary)

    def test_api_failure_never_authorizes_deployment(self):
        result, output, _, _ = self.check_run(exit_code=1)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(output, '')

    def test_missing_or_invalid_latest_run_fails_closed(self):
        for value in ['', 'null', 'unexpected', '120\n121']:
            with self.subTest(value=value):
                result, output, _, _ = self.check_run(latest=value)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(output, '')


if __name__ == '__main__':
    unittest.main()
