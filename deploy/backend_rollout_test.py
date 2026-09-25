"""Real Compose parsing with simulated image/container/HTTP operations only."""
import base64
import json
import os
from pathlib import Path
import shutil
import sqlite3
import re
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
PREVIOUS, TARGET, UNUSED = ('1' * 40, '2' * 40, '3' * 40)
TOKEN = base64.urlsafe_b64encode(bytes(range(32))).decode().rstrip('=')
COMMAND = r'''#!/usr/bin/env python3
import json, os, pathlib, subprocess, sys
args=sys.argv[1:]; root=pathlib.Path.cwd(); kind=pathlib.Path(sys.argv[0]).name
mode=os.environ['ROLLOUT_TEST_MODE']; target=os.environ['ROLLOUT_TARGET']; previous=os.environ['ROLLOUT_PREVIOUS']
prefix='org.postpilot.media.'
running=json.loads((root/'running').read_text())
cfg={}; operation=''
if kind=='docker' and args[0]=='compose':
    if 'config' in args:
        os.execv(os.environ['ROLLOUT_REAL_DOCKER'], ['docker',*args])
    at=next(i for i,a in enumerate(args) if a in ['pull','up','logs','run','exec'])
    operation=args[at]
    cfg=json.loads(subprocess.check_output([os.environ['ROLLOUT_REAL_DOCKER'],*args[:at],'config','--format','json'],text=True))
images={k:v['image'] for k,v in cfg.get('services',{}).items()}
with (root/'events').open('a') as f:f.write(json.dumps({'kind':kind,'args':args,'images':images})+'\n')
def fail():sys.exit(1)
if kind=='curl':
    if mode=='rollback_failure' or (mode in ['health_failure','bootstrap_failure'] and running.get('api','').endswith(target)):fail()
elif args[:2]==['image','inspect']:
    ref=args[-1]
    if args[-2]=='{{.Created}}':print('2020-01-01T00:00:00Z')
    else:
        labels={prefix+k:v for k,v in {'protocol':'1','renderer':'cpu-v1','assets':'assets-v1','asset-input-sha256':'a'*64,'rollback-safe':'1'}.items()}
        if mode in ['protocol_mismatch','asset_mismatch'] and 'postpilot-media-worker' in ref and ref.endswith(target):labels[prefix+('protocol' if mode=='protocol_mismatch' else 'assets')]='different'
        if mode in ['legacy_previous','bootstrap_failure'] and 'postpilot-api:' in ref and ref.endswith(previous):labels.pop(prefix+'rollback-safe')
        if mode=='legacy_target' and 'postpilot-api:' in ref and ref.endswith(target):labels.pop(prefix+'rollback-safe')
        print(json.dumps(labels))
elif args[:2]==['image','ls']:
    repo=next(a.split('=',1)[1] for a in args if a.startswith('reference='))
    for tag in [previous,target,'3'*40]:print(json.dumps({'Repository':repo,'Tag':tag}))
    print(json.dumps({'Repository':'another/project','Tag':'old'}))
elif args[0]=='ps':
    if args[-1]=='{{.Image}}':print('\n'.join(running.values()))
    elif running.get('media-worker'):print('abcdef123456')
elif args[0]=='stop':
    running.pop('media-worker',None);(root/'running').write_text(json.dumps(running))
elif args[0]=='run':
    if args[-1]=='status':
        if mode=='status_failure' and running.get('api','').endswith(target):fail()
        print('{"Ready":true,"Profile":"cpu","Waiting":0,"Active":0,"OwnActive":0}')
    else:
        if mode=='font_failure' and args[-1]=='media-manifest':fail()
        print(json.dumps({'ContractVersion':1,'RendererVersion':'cpu-v1','AssetVersion':'assets-v1','Profile':'cpu','RuntimeManifest':json.dumps({'Fonts':{'wanted':'hash'},'Overlays':'digest'})}))
elif operation=='pull':
    if mode=='pull_failure':fail()
elif operation=='up':
    service=args[-1];running[service]=images[service];(root/'running').write_text(json.dumps(running))
    if mode=='start_failure' and images[service].endswith(target):fail()
elif operation=='exec':
    if mode=='worker_health_failure' and running.get('media-worker','').endswith(target):fail()
    print('{"Ready":true}')
elif operation=='run':
    if mode=='remote_unreachable':fail()
    print('{"Ready":true}')
elif operation=='logs':
    print('migration failed: fixture diagnostic',file=sys.stderr)
'''


@unittest.skipUnless(shutil.which('docker'), 'Docker Compose is required')
class RolloutTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='postpilot-rollout-')
        self.addCleanup(self.directory.cleanup)
        self.stack = Path(self.directory.name)
        for name in ['docker-compose.prod.yml', 'docker-compose.media.colocated.yml', 'docker-compose.media.remote.yml', 'docker-compose.media.nvidia.yml']:
            shutil.copyfile(ROOT / name, self.stack / name)
        shutil.copyfile(ROOT / 'deploy/media/docker-compose.yml', self.stack / 'docker-compose.yml')
        shutil.copyfile(ROOT / 'deploy/media/docker-compose.nvidia.yml', self.stack / 'docker-compose.nvidia.yml')
        self.original = (
            f'IMAGE_TAG={PREVIOUS}\nAPI_UPSTREAM=postpilot-api-test\n'
            'CORS_ORIGIN=https://web.invalid\nMEDIA_TOPOLOGY=colocated\n'
            'MEDIA_WORKER_CREDENTIALS=' + json.dumps({'test-cpu': TOKEN}) + '\n'
            "# Preserve other settings, quotes and comments.\nTEST_SETTING='literal value # unchanged'\n"
        )
        worker = (ROOT / 'deploy/media/worker.env.example').read_text()
        self.worker_original = worker.replace('MEDIA_WORKER_IMAGE_TAG=\n', f'MEDIA_WORKER_IMAGE_TAG={PREVIOUS}\n').replace('MEDIA_WORKER_ID=prod-cpu-1', 'MEDIA_WORKER_ID=test-cpu').replace('MEDIA_WORKER_TOKEN=\n', f'MEDIA_WORKER_TOKEN={TOKEN}\n')
        self.write_env()
        (self.stack / 'running').write_text(json.dumps({'api': 'ghcr.io/hetarho/postpilot-api:' + PREVIOUS, 'media-worker': 'ghcr.io/hetarho/postpilot-media-worker:' + PREVIOUS}))
        commands = self.stack / 'bin'; commands.mkdir()
        for name in ['docker', 'curl']:
            path = commands / name; path.write_text(COMMAND); path.chmod(0o755)
        self.environment = dict(os.environ, PATH=str(commands) + os.pathsep + os.environ['PATH'],
            IMAGE_TAG=TARGET, MEDIA_WORKER_IMAGE_TAG=TARGET, API_ORIGIN='https://api.invalid',
            ROLLOUT_TARGET=TARGET, ROLLOUT_PREVIOUS=PREVIOUS, ROLLOUT_REAL_DOCKER=shutil.which('docker'))

    def write_env(self):
        for name, text in [('.env', self.original), ('worker.env', self.worker_original)]:
            p = self.stack / name; p.write_text(text); p.chmod(0o600)

    def remote(self):
        self.original = self.original.replace('MEDIA_TOPOLOGY=colocated', 'MEDIA_TOPOLOGY=remote')
        self.worker_original = self.worker_original.replace('http://api:9000', 'https://vps.example.ts.net:9443')
        self.write_env()
        (self.stack / 'running').write_text(json.dumps({'api': 'ghcr.io/hetarho/postpilot-api:' + PREVIOUS}))

    def run_rollout(self, mode='success', *args, role='api'):
        env = dict(self.environment, ROLLOUT_TEST_MODE=mode)
        result = subprocess.run(['python3', str(ROOT / 'deploy/media_rollout.py'), role, *args],
                                cwd=self.stack, env=env, capture_output=True, text=True, timeout=45)
        events_path = self.stack / 'events'
        events = [json.loads(line) for line in events_path.read_text().splitlines()] if events_path.exists() else []
        self.assertNotIn(TOKEN, result.stdout + result.stderr)
        self.assertNotIn('--remove-orphans', json.dumps(events))
        return result, events

    def assert_no_swap(self, events):
        self.assertFalse(any('up' in e['args'] or e['args'][0] == 'stop' for e in events))
        self.assertEqual((self.stack / '.env').read_text(), self.original)
        self.assertEqual((self.stack / 'worker.env').read_text(), self.worker_original)

    def uninitialized(self):
        self.original = re.sub(r'^MEDIA_(?:TOPOLOGY|WORKER_CREDENTIALS)=.*\n', '', self.original, flags=re.M)
        self.original += 'OPENROUTER_API_KEY=fixture-private-provider\n'
        self.write_env()
        (self.stack / 'worker.env').unlink()
        (self.stack / 'running').write_text(json.dumps({'api': 'ghcr.io/hetarho/postpilot-api:' + PREVIOUS}))

    def prepared_worker(self):
        return dict(line.split('=', 1) for line in (self.stack / 'worker.env').read_text().splitlines()
                    if line and not line.startswith('#'))

    def api_credentials(self):
        raw = re.search(r'^MEDIA_WORKER_CREDENTIALS=(.*)$', (self.stack / '.env').read_text(), re.M)[1]
        return json.loads(raw.strip("'"))

    def test_missing_worker_env_initializes_cpu_and_backs_up_live_wal_database(self):
        self.uninitialized()
        (self.stack / 'data').mkdir()
        database = self.stack / 'data/postpilot.db'
        with sqlite3.connect(database) as live:
            live.execute('PRAGMA journal_mode=WAL')
            live.execute('CREATE TABLE fixture (value TEXT)')
            live.execute("INSERT INTO fixture VALUES ('committed WAL content')")
            live.commit()
            self.assertTrue(Path(str(database) + '-wal').exists())
            result, events = self.run_rollout('legacy_previous', '--init-worker')
            self.assertEqual(result.returncode, 0, result.stderr)
            backup = self.stack / '.deploy/worker-init-before/database.sqlite3'
            with sqlite3.connect(backup) as saved:
                self.assertEqual(saved.execute('SELECT value FROM fixture').fetchall(), [('committed WAL content',)])
            self.assertEqual(live.execute('SELECT value FROM fixture').fetchall(), [('committed WAL content',)])
        worker = self.prepared_worker()
        self.assertEqual(worker['MEDIA_WORKER_VARIANT'], 'cpu')
        self.assertEqual(worker['MEDIA_ACCEL'], 'cpu')
        self.assertEqual(worker['MEDIA_API_URL'], 'http://api:9000')
        self.assertEqual(worker['MEDIA_WORKER_IMAGE_TAG'], TARGET)
        self.assertEqual(self.api_credentials()[worker['MEDIA_WORKER_ID']], worker['MEDIA_WORKER_TOKEN'])
        self.assertEqual(len(base64.urlsafe_b64decode(worker['MEDIA_WORKER_TOKEN'] + '=')), 32)
        self.assertNotIn(worker['MEDIA_WORKER_TOKEN'], result.stdout + result.stderr + json.dumps(events))
        self.assertNotIn('fixture-private-provider', (self.stack / 'worker.env').read_text())
        self.assertIn("TEST_SETTING='literal value # unchanged'", (self.stack / '.env').read_text())
        for name in ['.env', 'worker.env', '.deploy/worker-init-before/database.sqlite3']:
            self.assertEqual((self.stack / name).stat().st_mode & 0o777, 0o600)
        self.assertEqual((self.stack / '.deploy/worker-init-before/.env').read_text(), self.original)
        self.assertEqual([e['args'][-1] for e in events if 'up' in e['args']], ['api', 'media-worker'])

    def test_initial_failure_can_retry_without_rotating_credentials_or_booting_legacy(self):
        self.uninitialized()
        result, events = self.run_rollout('bootstrap_failure', '--init-worker')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('rollback rejected', result.stderr)
        self.assertTrue(all(e['images'][e['args'][-1]].endswith(TARGET) for e in events if 'up' in e['args']))
        before = self.prepared_worker()
        result, _ = self.run_rollout('success', '--init-worker')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.prepared_worker(), before)
        self.assertTrue((self.stack / '.deploy/last-good').exists())

    def test_retry_after_api_credentials_but_before_worker_file_reuses_token(self):
        self.uninitialized()
        result, _ = self.run_rollout('pull_failure', '--init-worker')
        self.assertNotEqual(result.returncode, 0)
        before = self.prepared_worker()
        (self.stack / 'worker.env').unlink()
        result, _ = self.run_rollout('legacy_previous', '--init-worker')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.prepared_worker()['MEDIA_WORKER_TOKEN'], before['MEDIA_WORKER_TOKEN'])
        self.assertEqual((self.stack / '.deploy/worker-init-before/.env').read_text(), self.original)

    def test_initialization_preserves_preconfigured_worker_credentials(self):
        (self.stack / 'worker.env').unlink()
        result, _ = self.run_rollout('success', '--init-worker')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.api_credentials()['test-cpu'], TOKEN)
        self.assertEqual(len(self.api_credentials()), 2)

    def test_initialization_leaves_existing_worker_settings_and_rollback_floor_intact(self):
        result, events = self.run_rollout('legacy_previous', '--init-worker')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('rollback floor', result.stderr)
        self.assert_no_swap(events)
        self.assertFalse((self.stack / '.deploy/worker-init-before').exists())

    def test_automatic_bootstrap_ends_after_first_healthy_deployment(self):
        self.uninitialized()
        result, _ = self.run_rollout('legacy_previous', '--init-worker')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.environment.update(IMAGE_TAG='4'*40, MEDIA_WORKER_IMAGE_TAG='4'*40,
                                ROLLOUT_TARGET='4'*40, ROLLOUT_PREVIOUS=TARGET)
        (self.stack / 'events').unlink()
        result, events = self.run_rollout('legacy_previous', '--init-worker')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('rollback floor', result.stderr)
        self.assertFalse(any('up' in e['args'] or e['args'][0]=='stop' for e in events))

    def test_deleted_worker_file_in_established_stack_is_not_regenerated(self):
        result, _ = self.run_rollout()
        self.assertEqual(result.returncode, 0, result.stderr)
        (self.stack / 'worker.env').unlink()
        before = (self.stack / '.env').read_text()
        result, _ = self.run_rollout('success', '--init-worker')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('established deployment', result.stderr)
        self.assertFalse((self.stack / 'worker.env').exists())
        self.assertEqual((self.stack / '.env').read_text(), before)

    def test_remote_initialization_is_not_guessed(self):
        self.remote()
        (self.stack / 'worker.env').unlink()
        result, events = self.run_rollout('success', '--init-worker')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('remote worker.env explicitly', result.stderr)
        self.assertEqual((self.stack / '.env').read_text(), self.original)
        self.assertFalse((self.stack / 'worker.env').exists())
        self.assertFalse(events)

    def test_failed_database_backup_keeps_existing_api_running(self):
        self.uninitialized()
        (self.stack / 'data').mkdir()
        database = self.stack / 'data/postpilot.db'
        database.write_bytes(b'not a SQLite database')
        result, events = self.run_rollout('legacy_previous', '--init-worker')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('SQLite backup failed', result.stderr)
        self.assertFalse(any('up' in e['args'] or e['args'][0]=='stop' for e in events))
        self.assertEqual(database.read_bytes(), b'not a SQLite database')
        self.assertFalse((self.stack / '.deploy/worker-init-before/database.sqlite3').exists())
        self.assertFalse(list((self.stack / '.deploy/worker-init-before').glob('.database-*')))

    def test_initialization_cannot_change_check_drain_or_rollback(self):
        self.uninitialized()
        for action in ['--check', '--drain', '--rollback']:
            result, events = self.run_rollout('success', '--init-worker', action)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('only supported for an ordinary API rollout', result.stderr)
            self.assertEqual((self.stack / '.env').read_text(), self.original)
            self.assertFalse((self.stack / 'worker.env').exists())
            self.assertFalse(events)

    def test_cpu_to_candidate_rollback_restores_cpu_without_device_override(self):
        result, _ = self.run_rollout()
        self.assertEqual(result.returncode, 0, result.stderr)
        worker = self.stack / 'worker.env'
        worker.write_text(worker.read_text().replace('MEDIA_WORKER_VARIANT=cpu', 'MEDIA_WORKER_VARIANT=nvidia-candidate'))
        result, _ = self.run_rollout()
        self.assertEqual(result.returncode, 0, result.stderr)
        (self.stack / 'events').unlink()
        result, events = self.run_rollout('success', '--rollback')
        self.assertEqual(result.returncode, 0, result.stderr)
        updates = [e for e in events if 'up' in e['args']]
        self.assertIn('MEDIA_WORKER_VARIANT=cpu', worker.read_text())
        self.assertNotIn('docker-compose.media.nvidia.yml', updates[-1]['args'])
        self.assertEqual(updates[-1]['images']['media-worker'], 'ghcr.io/hetarho/postpilot-media-worker:' + TARGET)

    def test_unknown_variant_is_rejected_before_container_changes(self):
        self.worker_original = self.worker_original.replace('MEDIA_WORKER_VARIANT=cpu', 'MEDIA_WORKER_VARIANT=guessed-cloud')
        self.write_env()
        result, events = self.run_rollout()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('MEDIA_WORKER_VARIANT', result.stderr)
        self.assert_no_swap(events)

    def test_candidate_pin_is_independent_and_snapshotted(self):
        self.worker_original = self.worker_original.replace('MEDIA_WORKER_VARIANT=cpu', 'MEDIA_WORKER_VARIANT=nvidia-candidate')
        self.write_env()
        result, events = self.run_rollout()
        self.assertEqual(result.returncode, 0, result.stderr)
        updates = [e for e in events if 'up' in e['args']]
        self.assertTrue(updates[-1]['images']['media-worker'].endswith('postpilot-media-worker-nvidia:' + PREVIOUS))
        self.assertTrue((self.stack / '.deploy/last-good/docker-compose.media.nvidia.yml').exists())
        result, _ = self.run_rollout('success', '--rollback')
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_remote_candidate_never_reserves_gpu_on_api(self):
        self.remote()
        self.worker_original = self.worker_original.replace('MEDIA_WORKER_VARIANT=cpu', 'MEDIA_WORKER_VARIANT=nvidia-candidate')
        self.write_env()
        result, events = self.run_rollout()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(any('docker-compose.media.nvidia.yml' in e['args'] for e in events))
        self.assertFalse(any('media-worker' in e['images'] for e in events))
        self.assertTrue(any(e['args'] == ['pull', 'ghcr.io/hetarho/postpilot-media-worker-nvidia:' + PREVIOUS] for e in events))

    def test_standalone_candidate_uses_explicit_new_pin(self):
        self.remote()
        self.worker_original = self.worker_original.replace('MEDIA_WORKER_VARIANT=cpu', 'MEDIA_WORKER_VARIANT=nvidia-candidate')
        self.write_env()
        result, events = self.run_rollout(role='worker')
        self.assertEqual(result.returncode, 0, result.stderr)
        updates = [e for e in events if 'up' in e['args']]
        self.assertTrue(updates[-1]['images']['media-worker'].endswith('postpilot-media-worker-nvidia:' + TARGET))
        self.assertTrue((self.stack / '.deploy/last-good/docker-compose.nvidia.yml').exists())

    def test_colocated_success_drains_then_starts_both_and_preserves_rollback_pins(self):
        result, events = self.run_rollout()
        self.assertEqual(result.returncode, 0, result.stderr)
        updates = [e for e in events if 'up' in e['args']]
        self.assertEqual([e['args'][-1] for e in updates], ['api', 'media-worker'])
        self.assertTrue(all(e['images'][e['args'][-1]].endswith(TARGET) for e in updates))
        stop_at = next(i for i, e in enumerate(events) if e['args'][0] == 'stop')
        self.assertLess(stop_at, events.index(updates[0]))
        self.assertEqual(events[stop_at]['args'][1:3], ['--time', '45'])
        self.assertEqual((self.stack / '.env').read_text(), self.original.replace(PREVIOUS, TARGET))
        self.assertEqual((self.stack / 'worker.env').read_text(), self.worker_original.replace(PREVIOUS, TARGET))
        removed = [e['args'][-1] for e in events if e['args'][:2] == ['image', 'rm']]
        self.assertEqual(len(removed), 2)
        self.assertTrue(all(ref.endswith(UNUSED) for ref in removed))
        self.assertEqual((self.stack / '.deploy/previous/.env').read_text(), self.original)
        self.assertEqual((self.stack / '.deploy/previous/worker.env').read_text(), self.worker_original)

    def test_remote_api_rollout_preserves_offline_worker_pin_and_starts_only_api(self):
        self.remote()
        result, events = self.run_rollout()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([e['args'][-1] for e in events if 'up' in e['args']], ['api'])
        self.assertEqual((self.stack / 'worker.env').read_text(), self.worker_original)
        self.assertFalse(any(e['args'][0] in ['ssh', 'stop'] for e in events))
        pull = next(e for e in events if 'pull' in e['args'] and e['args'][0] == 'compose')
        self.assertEqual(pull['args'][-1], 'api')

    def test_pull_failure_preserves_running_services_and_exact_env(self):
        result, events = self.run_rollout('pull_failure')
        self.assertNotEqual(result.returncode, 0)
        self.assert_no_swap(events)

    def assert_rollback(self, mode, healthy=True):
        result, events = self.run_rollout(mode)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.stack / '.env').read_text(), self.original)
        self.assertEqual((self.stack / 'worker.env').read_text(), self.worker_original)
        api_updates = [e['images']['api'] for e in events if 'up' in e['args'] and e['args'][-1] == 'api']
        self.assertEqual([v.rsplit(':', 1)[1] for v in api_updates], [TARGET, PREVIOUS])
        self.assertIn('migration failed: fixture diagnostic', result.stderr)
        self.assertIn('rollback healthy:' if healthy else 'rollback failed;', result.stderr)

    def test_health_failure_restores_both_envs_and_images(self):
        self.assert_rollback('health_failure')

    def test_start_failure_restores_both_envs_and_images(self):
        self.assert_rollback('start_failure')

    def test_worker_health_failure_restores_api_and_worker(self):
        self.assert_rollback('worker_health_failure')

    def test_private_status_failure_restores_api_and_worker(self):
        self.assert_rollback('status_failure')

    def test_failed_rollback_is_reported(self):
        self.assert_rollback('rollback_failure', healthy=False)

    def test_contract_and_runtime_failures_precede_downtime(self):
        for mode in ['protocol_mismatch', 'asset_mismatch', 'font_failure', 'legacy_target', 'legacy_previous']:
            with self.subTest(mode=mode):
                (self.stack / 'events').unlink(missing_ok=True)
                result, events = self.run_rollout(mode)
                self.assertNotEqual(result.returncode, 0)
                self.assert_no_swap(events)

    def test_first_forward_only_upgrade_never_boots_legacy_on_failure(self):
        result, events = self.run_rollout('bootstrap_failure', '--bootstrap')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('rollback rejected', result.stderr)
        self.assertEqual([e['images']['api'] for e in events if 'up' in e['args']], ['ghcr.io/hetarho/postpilot-api:' + TARGET])
        self.assertIn(TARGET, (self.stack / '.env').read_text())
        self.assertEqual((self.stack / '.deploy/bootstrap-before/.env').read_text(), self.original)

    def test_worker_secret_leak_and_unapproved_gpu_config_are_rejected_before_pull(self):
        for extra in ['DB_PATH=/private/db\n', 'MEDIA_ACCEL=nvenc\n', 'MEDIA_STOP_TIMEOUT=10\n']:
            with self.subTest(extra=extra):
                (self.stack / 'worker.env').write_text(self.worker_original + extra)
                result, events = self.run_rollout()
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(any('pull' in e['args'] for e in events))

    def test_check_only_does_not_swap_or_persist_incoming_tags(self):
        result, events = self.run_rollout('success', '--check')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_no_swap(events)

    def test_explicit_rollback_restores_saved_configs_without_exported_tag_override(self):
        result, _ = self.run_rollout()
        self.assertEqual(result.returncode, 0, result.stderr)
        (self.stack / 'events').unlink()
        result, events = self.run_rollout('success', '--rollback')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.stack / '.env').read_text(), self.original)
        self.assertEqual((self.stack / 'worker.env').read_text(), self.worker_original)
        self.assertTrue(all(e['images'][e['args'][-1]].endswith(PREVIOUS) for e in events if 'up' in e['args']))

    def test_standalone_worker_probes_actual_endpoint_before_drain(self):
        self.remote()
        (self.stack / 'running').write_text(json.dumps({'media-worker': 'ghcr.io/hetarho/postpilot-media-worker:' + PREVIOUS}))
        result, events = self.run_rollout(role='worker')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([e['args'][-1] for e in events if 'up' in e['args']], ['media-worker'])
        self.assertFalse(any(e['kind'] == 'curl' for e in events))
        probe_at = next(i for i, e in enumerate(events) if e['args'][0] == 'compose' and 'status' in e['args'])
        stop_at = next(i for i, e in enumerate(events) if e['args'][0] == 'stop')
        self.assertLess(probe_at, stop_at)

    def test_unreachable_api_leaves_remote_worker_running(self):
        self.remote()
        before = json.dumps({'media-worker': 'ghcr.io/hetarho/postpilot-media-worker:' + PREVIOUS})
        (self.stack / 'running').write_text(before)
        result, events = self.run_rollout('remote_unreachable', role='worker')
        self.assertNotEqual(result.returncode, 0)
        self.assert_no_swap(events)
        self.assertEqual((self.stack / 'running').read_text(), before)

    def test_failure_restores_last_successful_credentials_not_unapplied_manual_edits(self):
        result, _ = self.run_rollout()
        self.assertEqual(result.returncode, 0, result.stderr)
        saved = {name: (self.stack / name).read_text() for name in ['.env', 'worker.env']}
        changed_token = base64.urlsafe_b64encode(bytes(reversed(range(32)))).decode().rstrip('=')
        for name, content in saved.items():
            (self.stack / name).write_text(content.replace(TOKEN, changed_token))
        self.environment.update(IMAGE_TAG='4' * 40, MEDIA_WORKER_IMAGE_TAG='4' * 40, ROLLOUT_TARGET='4' * 40)
        (self.stack / 'events').unlink()
        result, _ = self.run_rollout('health_failure')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('rollback healthy:', result.stderr)
        for name, content in saved.items():
            self.assertEqual((self.stack / name).read_text(), content)

    def test_manual_rollback_can_recover_a_mistyped_current_env(self):
        result, _ = self.run_rollout()
        self.assertEqual(result.returncode, 0, result.stderr)
        (self.stack / '.env').write_text('IMAGE_TAG=broken\nMEDIA_WORKER_CREDENTIALS=not-json\n')
        (self.stack / 'worker.env').write_text('MEDIA_WORKER_TOKEN=broken\n')
        result, _ = self.run_rollout('success', '--rollback')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.stack / '.env').read_text(), self.original)
        self.assertEqual((self.stack / 'worker.env').read_text(), self.worker_original)

    def test_cleanup_keeps_another_stacks_selected_image(self):
        running = json.loads((self.stack / 'running').read_text())
        protected = 'ghcr.io/hetarho/postpilot-api:' + UNUSED
        running['staging-api'] = protected
        (self.stack / 'running').write_text(json.dumps(running))
        result, events = self.run_rollout()
        self.assertEqual(result.returncode, 0, result.stderr)
        removed = [e['args'][-1] for e in events if e['args'][:2] == ['image', 'rm']]
        self.assertNotIn(protected, removed)


if __name__ == '__main__':
    unittest.main()
