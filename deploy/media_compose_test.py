"""Resolve each supported CPU topology without credentials or live containers."""
import base64
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
import yaml

ROOT = Path(__file__).resolve().parents[1]


@unittest.skipUnless(shutil.which('docker'), 'Docker Compose is required')
class MediaComposeTest(unittest.TestCase):
    def test_colocated_remote_and_standalone_are_private_cpu_stacks(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ['docker-compose.prod.yml', 'docker-compose.media.colocated.yml', 'docker-compose.media.remote.yml']:
                shutil.copy2(ROOT / name, root / name)
            shutil.copy2(ROOT / 'deploy/media/docker-compose.yml', root / 'docker-compose.yml')
            token = base64.urlsafe_b64encode(bytes(range(32))).decode().rstrip('=')
            (root / '.env').write_text('IMAGE_TAG=' + '1' * 40 + '\nAPI_UPSTREAM=fixture-api\nDB_PATH=/data/private.db\nOPENROUTER_API_KEY=fixture-provider-secret\n')
            worker = (ROOT / 'deploy/media/worker.env.example').read_text().replace('MEDIA_WORKER_IMAGE_TAG=\n', 'MEDIA_WORKER_IMAGE_TAG=' + '2' * 40 + '\n').replace('MEDIA_WORKER_TOKEN=\n', 'MEDIA_WORKER_TOKEN=' + token + '\n')
            (root / 'worker.env').write_text(worker)
            for topology in ['colocated', 'remote', 'worker']:
                with self.subTest(topology=topology):
                    args = ['docker', 'compose', '--env-file', '.env', '--env-file', 'worker.env']
                    files = ['docker-compose.yml'] if topology == 'worker' else ['docker-compose.prod.yml', f'docker-compose.media.{topology}.yml']
                    for name in files:
                        args += ['-f', name]
                    cfg = json.loads(subprocess.check_output(args + ['config', '--format', 'json'], cwd=root, text=True))
                    if topology != 'worker':
                        api = cfg['services']['api']
                        self.assertEqual(api['environment']['MEDIA_INTERNAL_ADDR'], ':9000')
                        self.assertEqual(api['volumes'][0]['target'], '/data')
                        ports = api.get('ports', [])
                        if topology == 'remote':
                            self.assertEqual([(p['host_ip'], p['published'], p['target']) for p in ports], [('127.0.0.1', '9000', 9000)])
                            self.assertEqual(set(cfg['services']), {'api'})
                        else:
                            self.assertFalse(ports)
                    if topology != 'remote':
                        w = cfg['services']['media-worker']
                        self.assertFalse(w.get('ports'))
                        self.assertFalse(w.get('devices'))
                        self.assertFalse(w.get('deploy'))
                        self.assertEqual(w['environment']['MEDIA_ACCEL'], 'cpu')
                        self.assertFalse(any(k.startswith(('R2_', 'DB_', 'OPENROUTER_', 'TOSS_', 'PROVIDERS_')) for k in w['environment']))
                        self.assertEqual([v['target'] for v in w['volumes']], ['/var/lib/postpilot-media'])
                        self.assertNotIn('edge', w['networks'])
                        self.assertEqual(int(w['mem_limit']), 512 << 20)
                        self.assertEqual(w['memswap_limit'], w['mem_limit'])
                        self.assertEqual(w['stop_grace_period'], '45s')

    def test_explicit_candidate_overrides_and_offline_diagnostics(self):
        gpu = ROOT / 'deploy/media/docker-compose.nvidia.yml'
        self.assertEqual(gpu.read_text(), (ROOT / 'docker-compose.media.nvidia.yml').read_text())
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ['docker-compose.prod.yml', 'docker-compose.media.colocated.yml', 'docker-compose.media.nvidia.yml']:
                shutil.copy2(ROOT / name, root / name)
            for name in ['docker-compose.yml', 'docker-compose.nvidia.yml', 'docker-compose.diagnostics.yml']:
                shutil.copy2(ROOT / 'deploy/media' / name, root / name)
            (root / '.env').write_text('IMAGE_TAG=' + '1'*40 + '\nAPI_UPSTREAM=fixture\n')
            (root / 'worker.env').write_text('MEDIA_WORKER_IMAGE_TAG='+'2'*40+'\nMEDIA_ACCEL=cpu\nMEDIA_WORKER_TOKEN=fixture-secret\n')
            for files in [['docker-compose.prod.yml', 'docker-compose.media.colocated.yml', 'docker-compose.media.nvidia.yml'], ['docker-compose.yml', 'docker-compose.nvidia.yml'], ['docker-compose.diagnostics.yml']]:
                args=['docker','compose','--env-file','.env','--env-file','worker.env']
                for name in files: args += ['-f',name]
                cfg=json.loads(subprocess.check_output(args+['config','--format','json'],cwd=root,text=True))
                service='diagnostic' if len(files)==1 else 'media-worker'
                w=cfg['services'][service]
                self.assertIn('postpilot-media-worker-nvidia:',w['image'])
                devices=w['deploy']['resources']['reservations']['devices']
                self.assertEqual(devices,[{'driver':'nvidia','count':1,'capabilities':['gpu']}])
                self.assertEqual(w['environment']['NVIDIA_DRIVER_CAPABILITIES'],'compute,video,utility')
                if service=='diagnostic':
                    self.assertEqual(w['network_mode'],'none')
                    self.assertTrue(w['read_only'])
                    self.assertNotIn('MEDIA_WORKER_TOKEN',w['environment'])
                    self.assertNotIn('MEDIA_API_URL',w['environment'])

    def test_public_edge_routes_only_to_api_port(self):
        for file in (ROOT / 'deploy/edge/conf.d').glob('postpilot*.caddy*'):
            source = file.read_text()
            self.assertNotIn(':9000', source)
            self.assertIn(':8080', source)
        source = yaml.safe_load((ROOT / 'docker-compose.media.colocated.yml').read_text())
        self.assertEqual(source['services']['media-worker']['env_file'], 'worker.env')

    def test_both_images_are_published_and_rollout_never_connects_to_worker_host(self):
        workflow = yaml.safe_load((ROOT / '.github/workflows/deploy-backend.yml').read_text())
        builds = [s['with'] for s in workflow['jobs']['build']['steps'] if str(s.get('uses', '')).startswith('docker/build-push-action')]
        self.assertEqual({s['target'] for s in builds}, {'deployable', 'media-worker-deployable'})
        self.assertTrue(all('${{ github.sha }}' in s['tags'] and 'SOURCE_REVISION=${{ github.sha }}' == s['build-args'] for s in builds))
        steps = workflow['jobs']['rollout']['steps']
        for step in steps:
            if 'ssh-action' in step.get('uses', ''):
                self.assertEqual(step['with']['host'], '${{ secrets.SSH_HOST }}')
        smoke = workflow['jobs']['smoke']['steps']
        self.assertTrue(any(s.get('with', {}).get('target') == 'media-worker-smoke' for s in smoke))
        self.assertTrue(any('docker run' in s.get('run', '') and 'postpilot-worker-smoke:ci' in s['run'] for s in smoke))
