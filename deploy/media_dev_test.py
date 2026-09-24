"""CPU development config and image boundaries; no running installation is touched."""
import base64
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
import yaml

ROOT = Path(__file__).resolve().parents[1]


class MediaDevTest(unittest.TestCase):
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
        self.assertEqual(api['environment']['CLIP_MEDIA_EXECUTION'], 'worker')
        self.assertNotIn('ports', worker)
        self.assertFalse(any('9000' in port for port in api['ports']))
        self.assertNotIn('devices', worker)
        self.assertNotIn('deploy', worker)
        self.assertEqual(worker['environment']['MEDIA_ACCEL'], 'cpu')
        self.assertEqual(worker['depends_on']['backend']['condition'], 'service_healthy')
        self.assertEqual(worker['healthcheck']['test'], ['CMD', '/app/tmp/media-worker', 'health'])
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
