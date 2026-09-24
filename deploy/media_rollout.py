"""Explicit Linux Compose rollout; no provisioning, remote SSH or database writes.

Compose parses dotenv values. Commands never contain deployment secrets.
Restrictive snapshots preserve each service's configuration and rollback pin.
"""
import argparse
import base64
import contextlib
from datetime import datetime, timedelta, timezone
import fcntl
import json
import math
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import tempfile
from urllib.parse import urlsplit

API_IMAGE = 'ghcr.io/hetarho/postpilot-api'
WORKER_IMAGE = 'ghcr.io/hetarho/postpilot-media-worker'
LABEL = 'org.postpilot.media.'
CONTRACT_LABELS = ('protocol', 'renderer', 'assets', 'asset-input-sha256')
TOOLS = {'CLIP_WORK_ROOT', 'CLIP_WORK_STALE_AGE', 'CLIP_MEDIA_TIMEOUT',
         'CLIP_FFMPEG_PATH', 'CLIP_FFPROBE_PATH', 'CLIP_RESVG_PATH', 'CLIP_OVERLAY_DIR',
         'CLIP_FONT_PATH', 'CLIP_FONT_PAPERLOGY_PATH', 'CLIP_FONT_JUA_PATH',
         'CLIP_FONT_NANUM_MYEONGJO_PATH', 'CLIP_FONT_NANUM_MYEONGJO_BOLD_PATH',
         'CLIP_ENCODE_THREADS', 'CLIP_DECODE_THREADS'}
WORKER_KEYS = TOOLS | {'MEDIA_WORKER_IMAGE_TAG', 'MEDIA_API_URL', 'MEDIA_WORKER_ID',
    'MEDIA_WORKER_TOKEN', 'MEDIA_ACCEL', 'MEDIA_WORKER_CONCURRENCY', 'MEDIA_WORKER_CPUS',
    'MEDIA_WORKER_MEMORY', 'MEDIA_DRAIN_TIMEOUT', 'MEDIA_STOP_TIMEOUT'}


class RolloutError(Exception):
    pass


def command(args, *, env=None, capture=False, timeout=300):
    try:
        result = subprocess.run(args, env=env, text=True, capture_output=capture, timeout=timeout)
    except (OSError, subprocess.TimeoutExpired) as error:
        raise RolloutError(f'{Path(args[0]).name} could not finish') from error
    if result.returncode:
        # Captured Compose config may contain secrets; do not echo it on error.
        raise RolloutError(f'{Path(args[0]).name} command failed (exit {result.returncode})')
    return result.stdout if capture else ''


def env_keys(path):
    return set(re.findall(r'^\s*(?:export\s+)?([A-Za-z_][A-Za-z_0-9]*)\s*=', path.read_text(), re.M))


def check_private_file(path):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_mode & 0o077:
        raise RolloutError(f'{path.name} must be a regular file with mode 0600')


def immutable_tag(tag):
    if not re.fullmatch(r'[a-f0-9]{40}', tag or ''):
        raise RolloutError('image tags must be published full 40-character commit SHAs')
    return tag


def duration(value):
    parts = re.findall(r'(\d+(?:\.\d+)?)(ms|s|m|h)', value)
    if ''.join(a + b for a, b in parts) != value or not parts:
        raise RolloutError('MEDIA_DRAIN_TIMEOUT must be a positive duration')
    return sum(float(a) * {'ms': .001, 's': 1, 'm': 60, 'h': 3600}[b] for a, b in parts)


def memory_bytes(value):
    m = re.fullmatch(r'(\d+)([kKmMgG])?', str(value))
    if not m:
        raise RolloutError('MEDIA_WORKER_MEMORY must use bytes or k/m/g units')
    return int(m[1]) * 1024 ** (' kmg'.index((m[2] or ' ').lower()))


class Stack:
    def __init__(self, role):
        self.role = role
        self.env_files = ['.env', 'worker.env'] if role == 'api' else ['worker.env']
        for name in self.env_files:
            check_private_file(Path(name))
        if env_keys(Path('worker.env')) - WORKER_KEYS:
            raise RolloutError('worker.env contains an unsupported setting or application secret')
        keys = set().union(*(env_keys(Path(p)) for p in self.env_files))
        self.environment = {k: v for k, v in os.environ.items()
                            if k not in keys | {'IMAGE_TAG', 'MEDIA_WORKER_IMAGE_TAG', 'MEDIA_TOPOLOGY',
                                               'COMPOSE_FILE', 'COMPOSE_PROFILES', 'COMPOSE_ENV_FILES'}}
        self.prefix = ['docker', 'compose']
        for name in self.env_files:
            self.prefix += ['--env-file', name]
        self.files = ['docker-compose.prod.yml'] if role == 'api' else ['docker-compose.yml']
        raw = self.compose('config', '--environment', capture=True)
        self.values = dict(line.split('=', 1) for line in raw.splitlines() if '=' in line)
        self.worker_values = {k: self.values[k] for k in env_keys(Path('worker.env'))}
        self.topology = self.values.get('MEDIA_TOPOLOGY', 'colocated') if role == 'api' else 'worker'
        if role == 'api':
            if self.topology not in ('colocated', 'remote'):
                raise RolloutError('MEDIA_TOPOLOGY must be colocated or remote')
            self.files.append(f'docker-compose.media.{self.topology}.yml')
        self.config = json.loads(self.compose('config', '--format', 'json', capture=True))
        self.project = self.config['name']
        self.services = ['api', 'media-worker'] if self.topology == 'colocated' else ['api' if role == 'api' else 'media-worker']
        self.worker_image = WORKER_IMAGE + ':' + immutable_tag(self.worker_values.get('MEDIA_WORKER_IMAGE_TAG'))
        self.api_image = None
        if role == 'api':
            self.api_image = API_IMAGE + ':' + immutable_tag(self.values.get('IMAGE_TAG'))
            api = self.config['services']['api']
            for key in ['API_UPSTREAM', 'CORS_ORIGIN', 'MEDIA_WORKER_CREDENTIALS']:
                if not api['environment'].get(key):
                    raise RolloutError(f'required API setting {key} is missing')
            credentials = json.loads(api['environment']['MEDIA_WORKER_CREDENTIALS'])
            if credentials.get(self.worker_values.get('MEDIA_WORKER_ID')) != self.worker_values.get('MEDIA_WORKER_TOKEN'):
                raise RolloutError('API and worker credentials do not match')
            if not os.environ.get('API_ORIGIN'):
                raise RolloutError('API_ORIGIN is required for the public health gate')
        self.validate_worker()

    def compose(self, *args, capture=False, timeout=300):
        files = sum((['-f', p] for p in self.files), [])
        return command(self.prefix + files + list(args), env=self.environment, capture=capture, timeout=timeout)

    def validate_worker(self):
        v = self.worker_values
        if v.get('MEDIA_ACCEL', 'cpu') not in ('cpu', 'auto'):
            raise RolloutError('production NVIDIA activation is pending; MEDIA_ACCEL must be cpu or auto')
        if v.get('MEDIA_WORKER_CONCURRENCY', '1') != '1':
            raise RolloutError('MEDIA_WORKER_CONCURRENCY must be 1')
        if not re.fullmatch(r'[A-Za-z0-9_.-]{1,128}', v.get('MEDIA_WORKER_ID', '')):
            raise RolloutError('MEDIA_WORKER_ID is invalid')
        try:
            token = v.get('MEDIA_WORKER_TOKEN', '')
            secret = base64.urlsafe_b64decode(token + '=' * (-len(token) % 4))
            valid = re.fullmatch(r'[A-Za-z0-9_-]+', token) and 32 <= len(secret) <= 96 and len(set(secret)) >= 16
        except ValueError:
            valid = False
        if not valid:
            raise RolloutError('MEDIA_WORKER_TOKEN must contain at least 32 random base64url bytes')
        origin = v.get('MEDIA_API_URL', '')
        parsed = urlsplit(origin)
        if not parsed.hostname or parsed.username or parsed.query or parsed.fragment or parsed.path not in ('', '/'):
            raise RolloutError('MEDIA_API_URL must be a private HTTP(S) origin')
        if self.topology == 'colocated' and origin != 'http://api:9000':
            raise RolloutError('colocated MEDIA_API_URL must be http://api:9000')
        if self.topology != 'colocated' and parsed.scheme != 'https':
            raise RolloutError('remote MEDIA_API_URL requires the configured private HTTPS origin')
        cpu = float(v.get('MEDIA_WORKER_CPUS', '1'))
        memory = memory_bytes(v.get('MEDIA_WORKER_MEMORY', '512m'))
        if not math.isfinite(cpu) or not .1 <= cpu <= 64 or not 256 << 20 <= memory <= 64 << 30:
            raise RolloutError('worker limits must be 0.1..64 CPUs and 256 MiB..64 GiB memory')
        drain = duration(v.get('MEDIA_DRAIN_TIMEOUT', '30s'))
        self.stop_seconds = int(v.get('MEDIA_STOP_TIMEOUT', '45'))
        if not 0 < drain <= 900 or not drain + 15 <= self.stop_seconds <= 915:
            raise RolloutError('MEDIA_STOP_TIMEOUT must exceed the bounded drain by at least 15 seconds')

    def snapshot(self):
        return {p: Path(p).read_bytes() for p in self.env_files + self.files}

    def images(self):
        return {self.worker_image} | ({self.api_image} if self.api_image else set())


@contextlib.contextmanager
def temporary_env(values):
    name = None
    try:
        with tempfile.NamedTemporaryFile(mode='w', prefix='postpilot-media-', delete=False) as file:
            name = file.name
            for key, value in values.items():
                if '\n' in str(value) or '\r' in str(value):
                    raise RolloutError('multiline media settings are unsupported')
                file.write(f'{key}={value}\n')
        yield name
    finally:
        if name:
            Path(name).unlink(missing_ok=True)


def image_labels(image):
    return json.loads(command(['docker', 'image', 'inspect', '--format', '{{json .Config.Labels}}', image], capture=True)) or {}


def compatible_images(api, worker):
    a, w = image_labels(api), image_labels(worker)
    if a.get(LABEL + 'rollback-safe') != '1':
        raise RolloutError('API predates the minimum media rollback version (rollback-safe=1); legacy boot would fail parked jobs')
    if any(not a.get(LABEL + key) or a[LABEL + key] != w.get(LABEL + key) for key in CONTRACT_LABELS):
        raise RolloutError('API/worker protocol, renderer or asset contract mismatch')
    return {key: a[LABEL + key] for key in CONTRACT_LABELS}


def offline_manifest(image, values, api=False):
    env = {k: v for k, v in values.items() if k in TOOLS}
    env.update(CLIP_WORK_ROOT='/tmp/media-preflight', MEDIA_ACCEL='cpu')
    with temporary_env(env) as name:
        args = ['docker', 'run', '--rm', '--network', 'none', '--cpus', '1', '--memory', '256m',
                '--env-file', name, '--entrypoint', '/api' if api else '/media-worker', image,
                'media-manifest' if api else 'manifest']
        return json.loads(command(args, capture=True, timeout=60))


def validate_images(stack):
    if stack.role == 'api':
        contract = compatible_images(stack.api_image, stack.worker_image)
        profile = offline_manifest(stack.api_image, stack.config['services']['api']['environment'], api=True)
        if (str(profile['ContractVersion']), profile['RendererVersion'], profile['AssetVersion']) != tuple(contract[k] for k in CONTRACT_LABELS[:3]):
            raise RolloutError('API executable and image manifest disagree')
        if stack.topology == 'colocated':
            worker = offline_manifest(stack.worker_image, stack.worker_values)
            a, w = (json.loads(p['RuntimeManifest']) for p in (profile, worker))
            if a['Fonts'] != w['Fonts'] or a['Overlays'] != w['Overlays']:
                raise RolloutError('API/worker font or overlay overrides differ')
        return contract
    profile = offline_manifest(stack.worker_image, stack.worker_values)
    labels = image_labels(stack.worker_image)
    if any(not labels.get(LABEL + k) for k in CONTRACT_LABELS) or (str(profile['ContractVersion']), profile['RendererVersion'], profile['AssetVersion']) != tuple(labels[LABEL + k] for k in CONTRACT_LABELS[:3]):
        raise RolloutError('worker executable and image contract manifest disagree')
    return {k: labels.get(LABEL + k) for k in CONTRACT_LABELS}


def status_probe(stack):
    if stack.role == 'worker':
        raw = stack.compose('run', '--rm', '--no-deps', '-T', 'media-worker', 'status', capture=True, timeout=60)
    else:
        # The selected worker contract is checked on the API's private bridge.
        # The actual remote PC performs its own probe before its first activation.
        env = {k: stack.worker_values[k] for k in ('MEDIA_WORKER_ID', 'MEDIA_WORKER_TOKEN')}
        env.update(MEDIA_API_URL='http://api:9000', MEDIA_ACCEL='cpu', CLIP_WORK_ROOT='/tmp/media-probe')
        network = stack.config['networks']['media']['name']
        with temporary_env(env) as name:
            raw = command(['docker', 'run', '--rm', '--network', network, '--cpus', '1', '--memory', '256m',
                           '--env-file', name, '--entrypoint', '/media-worker', stack.worker_image, 'status'], capture=True, timeout=60)
    if not json.loads(raw).get('Ready'):
        raise RolloutError('compatible worker status smoke failed')


def drain(stack):
    ids = command(['docker', 'ps', '--filter', f'label=com.docker.compose.project={stack.project}',
                   '--filter', 'label=com.docker.compose.service=media-worker', '--format', '{{.ID}}'], capture=True).split()
    if ids:
        if any(not re.fullmatch(r'[a-f0-9]{12,64}', value) for value in ids):
            raise RolloutError('invalid worker container identity')
        command(['docker', 'stop', '--time', str(stack.stop_seconds), *ids], timeout=stack.stop_seconds + 30)


def public_health():
    command(['curl', '--retry', '15', '--retry-all-errors', '--retry-delay', '2', '--connect-timeout', '5',
             '--max-time', '10', '--fail', '--silent', '--show-error', '--output', '/dev/null',
             os.environ['API_ORIGIN'].rstrip('/') + '/health'], timeout=210)


def start(stack):
    if stack.role == 'api':
        stack.compose('up', '-d', '--no-deps', 'api')
        public_health()
        status_probe(stack)
    if 'media-worker' in stack.services:
        stack.compose('up', '-d', '--no-deps', '--wait', '--wait-timeout', '90', 'media-worker', timeout=120)
        stack.compose('exec', '-T', 'media-worker', '/media-worker', 'health', timeout=45)


def atomic_write(path, content):
    mode = stat.S_IMODE(path.stat().st_mode) if path.exists() else 0o600
    with tempfile.NamedTemporaryFile(dir=path.parent, prefix='.deploy-write-', delete=False) as file:
        temporary = Path(file.name)
        file.write(content)
    try:
        temporary.chmod(mode)
        temporary.replace(path)
    finally:
        temporary.unlink(missing_ok=True)


def restore(snapshot):
    for name, content in snapshot.items():
        atomic_write(Path(name), content)
        if name.endswith('.env'):
            Path(name).chmod(0o600)


def saved_snapshot(directory):
    if not directory.exists():
        return None
    names = json.loads((directory / 'files.json').read_text())
    allowed = {'.env', 'worker.env', 'docker-compose.yml', 'docker-compose.prod.yml',
               'docker-compose.media.colocated.yml', 'docker-compose.media.remote.yml'}
    if not set(names) <= allowed:
        raise RolloutError('invalid deployment snapshot')
    return {name: (directory / name).read_bytes() for name in names}


def save_snapshot(directory, snapshot, contract):
    directory.mkdir(mode=0o700, parents=True)
    for name, content in snapshot.items():
        path = directory / name
        path.write_bytes(content)
        path.chmod(0o600)
    (directory / 'files.json').write_text(json.dumps(list(snapshot)))
    (directory / 'contract.json').write_text(json.dumps(contract, sort_keys=True))


def set_tag(path, key, value):
    immutable_tag(value)
    source = path.read_text()
    pattern = rf'^(?:export\s+)?{key}=.*$'
    if re.search(pattern, source, flags=re.M):
        source = re.sub(pattern, f'{key}={value}', source, flags=re.M)
    else:
        source = source.rstrip('\n') + f'\n{key}={value}\n'
    atomic_write(path, source.encode())


def rollback_ready(role, snapshot):
    current = {name: Path(name).read_bytes() for name in snapshot if Path(name).exists()}
    try:
        restore(snapshot)
        old = Stack(role)
        if role == 'api':
            compatible_images(old.api_image, old.worker_image)
        else:
            status_probe(old)
        return old.images()
    except (RolloutError, ValueError, KeyError):
        return set()
    finally:
        restore(current)


def prune_images(protected):
    # Only this application's repositories, never the host's other projects.
    repositories = {ref.rsplit(':', 1)[0] for ref in protected}
    # Docker may remove one tag from a multiply-tagged live image without force.
    # Retain every container's selected ref, including another Postpilot stack.
    protected = protected | set(command(['docker', 'ps', '-a', '--format', '{{.Image}}'], capture=True).splitlines())
    cutoff = datetime.now(timezone.utc) - timedelta(hours=24)
    for repository in sorted(repositories & {API_IMAGE, WORKER_IMAGE}):
        rows = command(['docker', 'image', 'ls', '--filter', f'reference={repository}', '--format', '{{json .}}'], capture=True)
        for row in rows.splitlines():
            item = json.loads(row)
            ref = item['Repository'] + ':' + item['Tag']
            if item['Repository'] != repository or item['Tag'] == '<none>' or ref in protected:
                continue
            created = command(['docker', 'image', 'inspect', '--format', '{{.Created}}', ref], capture=True).strip()
            if datetime.fromisoformat(created.replace('Z', '+00:00')) < cutoff:
                # No --force: a running container is an additional hard guard.
                with contextlib.suppress(RolloutError):
                    command(['docker', 'image', 'rm', ref])


def rollout(args):
    good_dir = Path('.deploy/last-good')
    if args.rollback:
        previous = saved_snapshot(good_dir)
        target = saved_snapshot(Path('.deploy/previous'))
        if not previous or not target:
            raise RolloutError('no previous supported deployment snapshot')
        # Recovery must remain usable after a mistyped current env setting.
        current = {name: Path(name).read_bytes() for name in previous.keys() | target.keys() if Path(name).exists()}
    else:
        stack = Stack(args.role)
        if args.drain:
            drain(stack)
            return
        current = stack.snapshot()
        previous = saved_snapshot(good_dir) or current
    changed = False
    try:
        if args.rollback:
            restore(target)
        else:
            api_tag = immutable_tag(os.environ.get('IMAGE_TAG')) if args.role == 'api' else None
            worker_tag = immutable_tag(os.environ.get('MEDIA_WORKER_IMAGE_TAG')) if args.role == 'worker' or stack.topology == 'colocated' else None
            if api_tag:
                set_tag(Path('.env'), 'IMAGE_TAG', api_tag)
            if worker_tag:
                set_tag(Path('worker.env'), 'MEDIA_WORKER_IMAGE_TAG', worker_tag)
            # Remote API rollout preserves the executor's independently chosen pin.
        stack = Stack(args.role)
        stack.compose('pull', *stack.services)
        if stack.topology == 'remote':
            command(['docker', 'pull', stack.worker_image])
        contract = validate_images(stack)
        rollback_images = rollback_ready(args.role, previous)
        if not rollback_images and not args.bootstrap:
            raise RolloutError('previous images are below the compatible rollback floor; first upgrade requires the documented --bootstrap maintenance procedure')
        if args.role == 'worker':
            status_probe(stack)
        if args.check:
            restore(current)
            print('configuration and image contracts verified; no service changed; ' +
                  ('supported rollback verified' if rollback_images else 'legacy rollback unavailable (--bootstrap)'))
            return
        if not rollback_images and not Path('.deploy/bootstrap-before').exists():
            save_snapshot(Path('.deploy/bootstrap-before'), current, {})
        changed = True
        drain(stack)
        start(stack)
    except (OSError, RolloutError, ValueError, KeyError) as error:
        if not changed:
            restore(current)
            raise
        print(f'{error}; capturing startup logs', file=sys.stderr)
        with contextlib.suppress(RolloutError):
            stack.compose('logs', '--no-color', '--tail', '80', *stack.services)
        with contextlib.suppress(RolloutError):
            drain(stack)
        if not rollback_images:
            raise RolloutError('rollback rejected: legacy API boot is unsafe; keep the new configuration and fix forward in maintenance') from error
        restore(previous)
        try:
            old = Stack(args.role)
            start(old)
            print('rollback healthy: previous service images and env files restored', file=sys.stderr)
        except (RolloutError, ValueError, KeyError) as recovery:
            raise RolloutError('rollback failed; deployment needs recovery using .deploy snapshots') from recovery
        raise RolloutError('new deployment failed; supported rollback completed') from error
    next_dir = Path('.deploy/next')
    shutil.rmtree(next_dir, ignore_errors=True)
    save_snapshot(next_dir, stack.snapshot(), contract)
    previous_dir = Path('.deploy/previous')
    shutil.rmtree(previous_dir, ignore_errors=True)
    if good_dir.exists():
        good_dir.rename(previous_dir)
    elif rollback_images:
        save_snapshot(previous_dir, previous, contract)
    next_dir.rename(good_dir)
    try:
        prune_images(stack.images() | rollback_images)
    except (RolloutError, ValueError, KeyError) as error:
        print(f'deployment is healthy; image cleanup needs retry: {error}', file=sys.stderr)
    print(f'deployment healthy: {stack.topology}; snapshots retained in .deploy')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('role', choices=['api', 'worker'])
    action = parser.add_mutually_exclusive_group()
    action.add_argument('--check', action='store_true')
    action.add_argument('--drain', action='store_true')
    action.add_argument('--rollback', action='store_true')
    parser.add_argument('--bootstrap', action='store_true', help='first forward-only upgrade during operator maintenance')
    args = parser.parse_args()
    os.umask(0o077)
    Path('.deploy').mkdir(mode=0o700, exist_ok=True)
    Path('.deploy').chmod(0o700)
    try:
        with Path('.deploy/lock').open('w') as lock:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            rollout(args)
    except (OSError, RolloutError, ValueError, KeyError) as error:
        print(f'rollout stopped: {error}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
