"""CPU development config and image boundaries; no running installation is touched."""
import base64
import json
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest
import yaml

ROOT = Path(__file__).resolve().parents[1]


class MediaDevTest(unittest.TestCase):
    def assert_health_crypto_pin(self, source):
        instructions = [line.strip() for line in source.replace('\\\n', '').splitlines()]
        runs = [line for line in instructions if line.startswith('RUN ')]
        health = [line for line in runs if './cmd/media-health' in line]
        self.assertEqual(len(health), 1)
        snapshot = 'v1.0.0-c2097c7c'
        checksum = 'daf3614e0406f67ae6323c902db3f953a1effb199142362a039e7526dfb9368b'
        self.assertIn(f'GOFIPS140={snapshot} CGO_ENABLED=0', health[0])
        self.assertIn(f'/lib/fips140/{snapshot}.zip', health[0])
        self.assertIn(checksum, health[0])
        self.assertIn('sha256sum -c -', health[0])
        self.assertIn('LICENSE', health[0])
        self.assertEqual([line for line in runs if 'GOFIPS140=' in line], health)
        self.assertFalse(any(re.match(r'^(ENV|ARG)\s', line) and 'GOFIPS140' in line for line in instructions))

    def test_health_crypto_snapshot_is_verified_and_scoped_to_the_client(self):
        for name in ['Dockerfile', 'Dockerfile.dev']:
            with self.subTest(name=name):
                source = (ROOT / 'backend' / name).read_text()
                self.assert_health_crypto_pin(source)
                # An unpinned alias, changed archive or global build setting can
                # recreate the expensive health process or change other roles.
                mutations = {
                    'missing': source.replace('GOFIPS140=v1.0.0-c2097c7c ', ''),
                    'alias': source.replace('GOFIPS140=v1.0.0-c2097c7c ', 'GOFIPS140=v1.0.0 '),
                    'checksum': source.replace('daf3614e0406f67ae6323c902db3f953a1effb199142362a039e7526dfb9368b', '0' * 64),
                    'global': source + '\nENV GOFIPS140=v1.0.0-c2097c7c\n',
                    'other-role': source + '\nRUN GOFIPS140=v1.0.0-c2097c7c go build ./cmd/api\n',
                }
                for mutation, invalid in mutations.items():
                    with self.subTest(mutation=mutation):
                        with self.assertRaises(AssertionError):
                            self.assert_health_crypto_pin(invalid)

    def test_services_share_sources_but_not_database_secrets_or_work(self):
        cfg = yaml.safe_load((ROOT / 'docker-compose.yml').read_text())
        api, worker = cfg['services']['backend'], cfg['services']['media-worker']
        self.assertEqual(worker['command'], ['air', '-c', '.air-worker.toml'])
        self.assertEqual(worker['env_file'], [{'path': '.env.media.dev', 'required': True}])
        self.assertNotIn('backend_data:/app/data', worker['volumes'])
        self.assertNotIn('backend_tmp:/app/tmp', worker['volumes'])
        self.assertIn('media_worker_work:/var/lib/postpilot-media', worker['volumes'])
        self.assertFalse(any(k.startswith(('R2_', 'DB_', 'PROVIDERS')) for k in worker['environment']))
        self.assertEqual(api['environment']['MEDIA_STORAGE_ENDPOINT'], 'http://minio:9000')
        self.assertEqual(api['environment']['MEDIA_INTERNAL_ADDR'], ':9000')
        self.assertNotIn('ports', worker)
        self.assertFalse(any('9000' in port for port in api['ports']))
        self.assertNotIn('devices', worker)
        self.assertNotIn('deploy', worker)
        self.assertEqual(worker['environment']['MEDIA_ACCEL'], 'cpu')
        self.assertEqual(worker['depends_on']['backend']['condition'], 'service_healthy')
        self.assertEqual(worker['healthcheck']['test'], ['CMD', '/usr/local/bin/media-health'])
        self.assertEqual(api['develop']['watch'], worker['develop']['watch'])
        watched = {r['path'] for r in worker['develop']['watch'] if r['action'] == 'rebuild'}
        self.assertTrue({'./backend/assets', './backend/build', './backend/internal/clip/overlay', './backend/internal/clip/design'} <= watched)

    @unittest.skipUnless(shutil.which('docker'), 'Docker Compose is not installed')
    def test_actual_compose_resolves_both_cpu_services_with_fixture_credentials(self):
        with tempfile.TemporaryDirectory(prefix='postpilot-dev-config-') as directory:
            root = Path(directory)
            shutil.copy2(ROOT / 'docker-compose.yml', root / 'docker-compose.yml')
            for path in ['backend/assets', 'backend/build', 'backend/internal/clip/overlay', 'backend/internal/clip/design']:
                (root / path).mkdir(parents=True, exist_ok=True)
            shutil.copy2(ROOT / 'backend/Dockerfile.dev', root / 'backend/Dockerfile.dev')
            token = base64.urlsafe_b64encode(bytes(range(32))).decode().rstrip('=')
            (root / '.env.media.dev').write_text(f'MEDIA_API_URL=http://backend:9000\nMEDIA_WORKER_ID=dev-cpu\nMEDIA_WORKER_TOKEN={token}\nMEDIA_WORKER_CREDENTIALS=' + json.dumps({'dev-cpu': token}) + '\n')
            raw = subprocess.check_output(['docker', 'compose', '--profile', 'dev', 'config', '--format', 'json'], cwd=root, text=True)
            cfg = json.loads(raw)
            self.assertIn('media-worker', cfg['services'])
            env = cfg['services']['media-worker']['environment']
            self.assertEqual(env['MEDIA_WORKER_TOKEN'], token)
            self.assertFalse(any(k.startswith(('R2_', 'DB_', 'PROVIDERS')) for k in env))
            self.assertEqual(json.loads(cfg['services']['backend']['environment']['MEDIA_WORKER_CREDENTIALS'])['dev-cpu'], token)

    def test_worker_image_is_a_separate_nonroot_target_and_default_stays_api(self):
        source = (ROOT / 'backend/Dockerfile').read_text()
        worker = source.split('FROM media-runtime AS media-worker-runtime', 1)[1].split('FROM deployable AS production', 1)[0]
        self.assertNotIn('/api', worker)
        self.assertNotIn('PROVIDERS_CONFIG', worker)
        self.assertIn('AS media-worker-deployable', worker)
        self.assertIn('AS media-worker-smoke', worker)
        self.assertIn('RUN ["/media-worker", "manifest"]', worker)
        self.assertIn('USER nonroot:nonroot', source.split('FROM media-runtime AS runtime', 1)[0])
        self.assertEqual([line for line in source.splitlines() if line.startswith('FROM ')][-1], 'FROM deployable AS production')
