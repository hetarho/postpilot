#!/usr/bin/env python3
"""Disposable CPU release: no production env, ports, volumes or Docker socket mount."""
import argparse
import json
import math
import pathlib
import re
import secrets
import subprocess
import tempfile
import time
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[1]
STORAGE_IMAGE = 'postpilot:gate-media-storage'

def run(*args, check=True):
    p = subprocess.run(['docker', *args], cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=120)
    if check and p.returncode:
        raise RuntimeError(p.stdout[-6000:])
    return p.stdout.strip()

def build(target, tag, dockerfile='backend/Dockerfile'):
    subprocess.run(['docker', 'build', '--target', target, '-t', tag, '-f', dockerfile, '.'], cwd=ROOT, check=True, timeout=1800)

def validate_budget(args):
    if args.api_mib + args.worker_mib + args.reserve_mib > args.envelope_mib:
        raise RuntimeError('insufficient envelope: API + worker + cohost reserve exceeds memory')
    if args.api_cpus + args.worker_cpus > args.envelope_cpus:
        raise RuntimeError('insufficient CPU envelope: API + worker exceeds selected total')

def request(port, method='GET'):
    req = urllib.request.Request(f'http://127.0.0.1:{port}/control', method=method)
    with urllib.request.urlopen(req, timeout=3) as response:
        return json.load(response)

def fixture(layout, args):
    prefix = 'postpilot-release-' + secrets.token_hex(5)
    api, worker, store, relay = [prefix+'-'+x for x in ('api','worker','storage','relay')]
    private, remote = prefix+'-private', prefix+'-remote'
    containers, networks, volumes = [], [], []
    samples = {api: {'memory_peak': 0, 'disk_bytes': 0}, worker: {'memory_peak': 0, 'disk_bytes': 0}}
    # 768MiB execution + 256MiB stated cohost reserve, two CPUs total. MinIO
    # stands in for external R2; the network relay is separately reported infra.
    validate_budget(args)
    token = secrets.token_urlsafe(32)
    def launch(name, image, options, command=()):
        containers.append(name)
        return run('run','-d','--name',name,'--label','postpilot.disposable-media-release='+prefix,*options,image,*command)
    def snapshot(name):
        output=run('exec','-e','MEDIA_RELEASE_RESOURCE=1',name,'/clip-release.test','-test.run=^TestMediaReleaseResources$','-test.v',check=False)
        match=re.search(r'RESOURCE_REPORT (\{.*\})',output)
        if match:
            for key,value in json.loads(match[1]).items(): samples[name][key]=max(samples[name][key],value)
    try:
        for net in (private, remote) if layout=='remote' else (private,):
            run('network','create',*(['--internal'] if net==remote else []),net);networks.append(net)
        launch(store,STORAGE_IMAGE,['--network',private,'--network-alias','minio','--memory','256m','--cpus','.5','-e','MINIO_ROOT_USER=fixture','-e','MINIO_ROOT_PASSWORD=fixture-only-password'],['server','/data'])
        # Name the init utility too, so a CLI timeout cannot leave an unowned container.
        initializer=prefix+'-init';containers.append(initializer)
        # mc performs private bucket initialization with explicit, disposable keys.
        for _ in range(40):
            result=run('run','--rm','--name',initializer,'--label','postpilot.disposable-media-release='+prefix,'--network',private,'--entrypoint','/bin/sh',STORAGE_IMAGE,'-c','mc alias set fixture http://minio:9000 fixture fixture-only-password && mc mb --ignore-existing fixture/release && mc anonymous set none fixture/release',check=False)
            if 'Access permission' in result: break
            time.sleep(.5)
        else: raise RuntimeError('private MinIO initialization failed: '+result)
        volume=prefix+'-worker-work';run('volume','create',volume);volumes.append(volume)
        with tempfile.TemporaryDirectory(prefix=prefix) as directory:
            envfile=pathlib.Path(directory)/'api.env'
            values={'PORT':'8080','MEDIA_INTERNAL_ADDR':':9000','MEDIA_WORKER_CREDENTIALS':json.dumps({'release-cpu':token}), 'CLIP_MEDIA_LEASE_TTL':'8s','CLIP_MEDIA_WAIT_TIMEOUT':'10m','CLIP_MEDIA_STAGE_TIMEOUT':'20m','CLIP_MEDIA_MAX_ATTEMPTS':'3','R2_ENDPOINT':'http://127.0.0.1:8081','R2_PUBLIC_ENDPOINT':'http://minio:9000','MEDIA_STORAGE_ENDPOINT':'http://relay:9001' if layout=='remote' else 'http://minio:9000','R2_ACCESS_KEY_ID':'fixture','R2_SECRET_ACCESS_KEY':'fixture-only-password','R2_BUCKET':'release','MEDIA_RELEASE_LAYOUT':layout}
            envfile.write_text(''.join(k+'='+v+'\n' for k,v in values.items()));envfile.chmod(0o600)
            launch(api,args.api_image,['--network',private,'--network-alias','api','--memory',str(args.api_mib)+'m','--memory-swap',str(args.api_mib)+'m','--cpus',str(args.api_cpus),'--env-file',str(envfile),'-p','127.0.0.1::8081'])
        port=int(run('port',api,'8081/tcp').rsplit(':',1)[1])
        if layout=='remote':
            launch(relay,args.api_image,['--network',private,'--network-alias','relay','--memory','64m','--cpus','.25','-e','MEDIA_RELEASE_RELAY=1','--entrypoint','/clip-release.test'],['-test.run=^TestMediaReleaseRelay$','-test.timeout=30m'])
            run('network','connect','--alias','relay',remote,relay)
        worker_started=False
        deadline=time.monotonic()+args.timeout
        last_sample=0
        while time.monotonic()<deadline:
            status=json.loads(run('inspect',api))[0]['State']
            if not status['Running']:
                logs=run('logs',api,check=False)
                print(logs)
                if status['ExitCode']!=0: raise RuntimeError(('insufficient API memory (OOM)' if status['OOMKilled'] else 'release fixture failed')+': '+layout)
                if 'MEDIA_RELEASE_REPORT ' not in logs: raise RuntimeError('fixture report missing')
                report={'layout':layout,'scope':'synthetic execution envelope; host identity and available production capacity are not inferred','limits_mib':{'api':args.api_mib,'worker':args.worker_mib,'cohost_reserve':args.reserve_mib,'envelope':args.envelope_mib},'limits_cpus':{'api':args.api_cpus,'worker':args.worker_cpus,'envelope':args.envelope_cpus},'execution':samples,'combined_peak_upper_bound_bytes':sum(v['memory_peak'] for v in samples.values()),'infra':'private MinIO 256MiB/.5CPU and remote relay 64MiB/.25CPU, outside execution envelope (external R2 analog)'}
                if any(v['memory_peak']==0 for v in samples.values()):raise RuntimeError('resource evidence missing')
                print('MEDIA_RESOURCE_REPORT '+json.dumps(report))
                return report
            try: action=request(port).get('action')
            except (OSError,ValueError): action=None
            if action=='start':
                if worker_started: run('start',worker)
                else:
                    launch(worker,args.worker_image,['--network',remote if layout=='remote' else private,'--memory',str(args.worker_mib)+'m','--memory-swap',str(args.worker_mib)+'m','--cpus',str(args.worker_cpus),'-v',volume+':/var/lib/postpilot-media','-e','MEDIA_API_URL=http://'+('relay' if layout=='remote' else 'api')+':9002','-e','MEDIA_WORKER_ID=release-cpu','-e','MEDIA_WORKER_TOKEN='+token,'-e','MEDIA_ACCEL=cpu'])
                    worker_started=True
                request(port,'POST')
            elif action in ('kill','stop'):
                snapshot(worker)
                snapshot(api)
                run('kill',worker) if action=='kill' else run('stop','--time','40',worker)
                request(port,'POST')
            if time.monotonic()-last_sample>3:
                snapshot(api)
                if worker_started:
                    worker_state=json.loads(run('inspect',worker))[0]['State']
                    if worker_state['OOMKilled']:raise RuntimeError('insufficient worker memory: OOM gate failed')
                    if worker_state['Running']:snapshot(worker)
                last_sample=time.monotonic()
            time.sleep(.2)
        raise RuntimeError('bounded release timeout')
    finally:
        for name in containers:
            if name not in (store,prefix+'-init'): print(run('logs','--tail','30',name,check=False))
        for name in reversed(containers):run('rm','-f',name,check=False)
        for volume in volumes:run('volume','rm',volume,check=False)
        for network in reversed(networks):run('network','rm',network,check=False)

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--skip-build',action='store_true',help='reuse API/worker images; still prepare pinned fixture storage')
    parser.add_argument('--skip-storage-build',action='store_true',help='reuse the storage image explicitly built by the workflow')
    parser.add_argument('--layout',choices=['colocated','remote','both'],default='both')
    parser.add_argument('--api-image',default='postpilot:gate-media-release')
    parser.add_argument('--worker-image',default='postpilot:gate-media-release-worker')
    parser.add_argument('--api-mib',type=int,default=256)
    parser.add_argument('--worker-mib',type=int,default=512)
    parser.add_argument('--reserve-mib',type=int,default=256)
    parser.add_argument('--envelope-mib',type=int,default=1024)
    parser.add_argument('--api-cpus',type=float,default=1)
    parser.add_argument('--worker-cpus',type=float,default=1)
    parser.add_argument('--envelope-cpus',type=float,default=2)
    parser.add_argument('--timeout',type=int,default=1500)
    args=parser.parse_args()
    if min(args.api_mib,args.worker_mib,args.reserve_mib,args.envelope_mib,args.timeout)<=0:parser.error('budgets must be positive')
    if any(not math.isfinite(v) or v<=0 for v in (args.api_cpus,args.worker_cpus,args.envelope_cpus)):parser.error('CPU budgets must be finite and positive')
    validate_budget(args)
    # Never rely on a developer's cached, no-longer-public MinIO image. Fail
    # before application builds or disposable resources if source build fails.
    if args.skip_storage_build:
        run('image', 'inspect', STORAGE_IMAGE)
    else:
        build('media-storage', STORAGE_IMAGE, 'deploy/media/fixture.Dockerfile')
    if not args.skip_build:
        build('media-release-api',args.api_image);build('media-release-worker',args.worker_image)
    for layout in ('colocated','remote') if args.layout=='both' else (args.layout,):fixture(layout,args)

if __name__=='__main__':main()
