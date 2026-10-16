"""Fast deployment must remain independent of full, separately reported media gates."""
import unittest
import json
import os
import subprocess
import tempfile
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / '.github/workflows/deploy-backend.yml'
VERIFY = ROOT / '.github/workflows/media-verify.yml'


def build_steps(job):
    return [step for step in job.get('steps', [])
            if str(step.get('uses', '')).startswith('docker/build-push-action')]


class WorkflowBoundaries(unittest.TestCase):
    def setUp(self):
        self.deploy = yaml.safe_load(WORKFLOW.read_text())
        self.verify = yaml.safe_load(VERIFY.read_text())
        self.jobs = self.deploy['jobs']
        self.media = self.verify['jobs']['verify']

    def test_deploy_contains_only_build_and_health_gated_rollout(self):
        self.assertEqual(set(self.jobs), {'build', 'rollout'})
        self.assertEqual(self.jobs['rollout']['needs'], 'build')
        self.assertEqual([s['with']['target'] for s in build_steps(self.jobs['build'])],
                         ['deployable', 'media-worker-deployable'])
        ci = yaml.safe_load((WORKFLOW.parent / 'ci.yml').read_text())
        checks = [s.get('run','') for s in ci['jobs']['backend']['steps']]
        self.assertTrue(any("unittest discover -s deploy -p '*_test.py'" in command for command in checks))
        self.assertFalse(any('unittest discover' in s.get('run','') for s in self.jobs['build']['steps']))

    def test_ci_backend_runs_the_formatter_check_before_vet(self):
        # ARCH-26, ARCH-31: CI runs the same gofmt gate the local verify does.
        ci = yaml.safe_load((WORKFLOW.parent / 'ci.yml').read_text())
        checks = [s.get('run', '') for s in ci['jobs']['backend']['steps']]
        self.assertIn('test -z "$(gofmt -l .)"', checks)
        self.assertLess(checks.index('test -z "$(gofmt -l .)"'), checks.index('go vet ./...'))

    def test_only_server_rollout_holds_the_non_cancellable_deployment_lock(self):
        self.assertNotIn('concurrency', self.deploy)
        build = self.jobs['build']['concurrency']
        rollout = self.jobs['rollout']['concurrency']
        media = self.verify['concurrency']
        self.assertTrue(build['cancel-in-progress'])
        self.assertFalse(rollout['cancel-in-progress'])
        self.assertTrue(media['cancel-in-progress'])
        self.assertEqual(len({c['group'] for c in [build, rollout, media]}), 3)

    def test_every_server_operation_follows_the_supersession_check(self):
        steps = self.jobs['rollout']['steps']
        guard = next(i for i, s in enumerate(steps) if s.get('id') == 'current')
        self.assertEqual(steps[guard]['run'], 'sh deploy/latest-rollout.sh')
        self.assertEqual(self.jobs['rollout']['permissions']['actions'], 'read')
        remote = [(i,s) for i,s in enumerate(steps) if s.get('uses','').startswith('appleboy/')]
        self.assertEqual(len(remote), 5)
        for index, step in remote:
            self.assertGreater(index, guard)
            self.assertEqual(step['if'], "steps.current.outputs.deploy == 'true'")

    def test_rollout_initializes_the_default_worker_explicitly(self):
        steps = self.jobs['rollout']['steps']
        scripts = [step.get('with', {}).get('script', '') for step in steps]
        self.assertTrue(any('sh deploy/backend-rollout.sh --init-worker' in script for script in scripts))
        self.assertFalse(any('sh deploy/backend-rollout.sh --bootstrap' in script for script in scripts))
        self.assertTrue(any('deploy/media/worker.env.example' in step.get('with', {}).get('source', '') for step in steps))

    def test_storage_range_probe_runs_after_health_and_fails_the_deploy(self):
        steps = self.jobs['rollout']['steps']
        health = next(i for i, step in enumerate(steps)
                      if 'health gate' in step.get('name', ''))
        probe = next(i for i, step in enumerate(steps)
                     if 'check-storage-cors' in step.get('with', {}).get('script', ''))
        self.assertGreater(probe, health)
        step = steps[probe]
        self.assertEqual(step['if'], steps[health]['if'])
        self.assertFalse(step.get('continue-on-error'))
        self.assertEqual(step['env']['WEB_ORIGIN'], '${{ vars.WEB_ORIGIN }}')
        self.assertIn('WEB_ORIGIN', step['with']['envs'].split(','))
        script = step['with']['script']
        with tempfile.TemporaryDirectory(prefix='postpilot-storage-cors-workflow-') as directory:
            root = Path(directory)
            docker = root / 'docker'
            docker.write_text('#!/usr/bin/env python3\n'
                              'import json, os, sys\n'
                              'open(os.environ["PROBE_ARGS"], "w").write(json.dumps(sys.argv[1:]))\n'
                              'sys.exit(int(os.environ["PROBE_EXIT"]))\n')
            docker.chmod(0o755)
            # Only replace the SSH host's working directory; execute the actual gate body.
            script = script.replace('cd "/srv/postpilot-${ENV_NAME}"', 'cd "$PROBE_DIR"')
            for exit_code in (0, 42):
                args_file = root / 'args.json'
                env = {**os.environ, 'PATH': f'{root}:{os.environ["PATH"]}',
                       'ENV_NAME': 'prod', 'WEB_ORIGIN': 'https://web.example',
                       'PROBE_DIR': directory, 'PROBE_ARGS': str(args_file),
                       'PROBE_EXIT': str(exit_code)}
                result = subprocess.run(['sh', '-c', script], env=env, capture_output=True, text=True)
                self.assertEqual(result.returncode, exit_code, result.stderr)
                self.assertEqual(json.loads(args_file.read_text()),
                                 ['compose', '-f', 'docker-compose.prod.yml', 'exec', '-T',
                                  'api', '/api', 'check-storage-cors', 'https://web.example'])
            args_file.unlink()
            env['WEB_ORIGIN'] = ''
            result = subprocess.run(['sh', '-c', script], env=env, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(args_file.exists())

    def test_media_follows_a_successful_trusted_build_and_checks_its_exact_revision(self):
        trigger = self.verify.get('on', self.verify.get(True))
        self.assertEqual(trigger['workflow_run'], {'workflows':['Deploy backend'], 'types':['completed'], 'branches':['main']})
        self.assertIn('workflow_dispatch', trigger)
        self.assertIn("github.event.workflow_run.conclusion == 'success'", self.media['if'])
        self.assertIn('github.event.workflow_run.head_repository.full_name == github.repository', self.media['if'])
        self.assertEqual(self.verify['permissions'], {'contents': 'read'})
        checkout = self.media['steps'][0]
        self.assertEqual(checkout['with']['ref'], '${{ github.event.workflow_run.head_sha || github.sha }}')
        self.assertFalse(checkout['with']['persist-credentials'])
        self.assertNotIn('secrets.', VERIFY.read_text())

    def test_all_three_media_gates_run_independently_and_report_failure(self):
        self.assertNotIn('needs', self.media)
        self.assertEqual(self.media['strategy']['matrix']['gate'], ['image','worker','release'])
        self.assertFalse(self.media['strategy']['fail-fast'])
        self.assertFalse(self.media.get('continue-on-error'))
        targets = {s['with']['target']:s for s in build_steps(self.media)}
        self.assertEqual(set(targets), {'production','media-worker-smoke','media-storage','media-release-api','media-release-worker'})
        self.assertEqual(targets['production']['if'], "matrix.gate == 'image'")
        self.assertEqual(targets['media-worker-smoke']['if'], "matrix.gate == 'worker'")
        for step in self.media['steps']:
            self.assertFalse(step.get('continue-on-error'))
        worker = next(s for s in self.media['steps'] if s.get('run') == 'docker run --rm --cpus=2 --memory=1g postpilot-worker-smoke:ci')
        self.assertEqual(worker['if'], "matrix.gate == 'worker'")

    def test_fixture_images_share_buildx_cache_and_are_not_rebuilt_by_the_runner(self):
        steps = self.media['steps']
        targets = {s['with']['target']:s['with'] for s in build_steps(self.media)}
        for target, tag in [('media-storage','postpilot:gate-media-storage'), ('media-release-api','postpilot:gate-media-release'), ('media-release-worker','postpilot:gate-media-release-worker')]:
            self.assertTrue(targets[target]['load'])
            self.assertEqual(targets[target]['tags'], tag)
            self.assertIn('type=gha', targets[target]['cache-from'])
        self.assertIn('scope=backend-api', targets['media-release-api']['cache-from'])
        self.assertIn('scope=backend-worker', targets['media-release-worker']['cache-from'])
        self.assertTrue(any(s.get('run') == 'python3 scripts/media-release.py --skip-build --skip-storage-build' for s in steps))
        self.assertFalse(any('docker build ' in s.get('run','') for s in steps))
        scopes = [s['with']['cache-to'] for s in build_steps(self.jobs['build'])]
        self.assertEqual(len(set(scopes)), 2)


if __name__ == '__main__':
    unittest.main()
