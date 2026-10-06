"""Offline copy comparison using the immutable image that executes native media.

Inputs are synthetic browser artifacts and fixture references. This tool neither
contacts a provider nor qualifies semantic understanding or device throughput.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess

parser = argparse.ArgumentParser()
parser.add_argument('--copies', type=Path, required=True)
parser.add_argument('--fixtures', type=Path, required=True)
parser.add_argument('--output', type=Path, required=True)
parser.add_argument('--image', required=True)
args = parser.parse_args()
args.output.mkdir(parents=True, exist_ok=True)
commands = []


def execute(binary, command):
    cmd = ['docker', 'run', '--rm', '--network', 'none', '--memory', '1g', '--memory-swap', '1g', '--cpus', '2',
           '--mount', f'type=bind,src={args.copies.resolve()},dst=/copies,readonly',
           '--mount', f'type=bind,src={args.fixtures.resolve()},dst=/fixtures,readonly',
           '--mount', f'type=bind,src={args.output.resolve()},dst=/out',
           '--entrypoint', '/usr/local/bin/' + binary, args.image, *command]
    p = subprocess.run(cmd, capture_output=True, timeout=180, check=False)
    commands.append({'command': cmd, 'exit': p.returncode, 'stderr': p.stderr.decode()[:4096],
                     'stdoutSHA256': hashlib.sha256(p.stdout).hexdigest(), 'stdoutBytes': len(p.stdout)})
    if p.returncode != 0:
        raise RuntimeError(p.stderr.decode())
    if len(p.stdout) > 8 * 1024 * 1024:
        raise RuntimeError('Diagnostic output exceeds bound')
    return p.stdout


results = []
for path in sorted(args.copies.glob('browser-*.mp4')):
    probe = json.loads(execute('ffprobe', ['-v', 'error', '-show_streams', '-show_format', '-show_frames', '-of', 'json', '/copies/' + path.name]))
    video = [s for s in probe['streams'] if s['codec_type'] == 'video']
    audio = [s for s in probe['streams'] if s['codec_type'] == 'audio']
    assert len(video) == 1 and len(audio) <= 1
    v = video[0]
    assert v['codec_name'] == 'h264' and v['pix_fmt'] == 'yuv420p' and v['sample_aspect_ratio'] == '1:1'
    assert v['avg_frame_rate'] == '15/1' and max(v['width'], v['height']) <= 720
    assert 0 < float(probe['format']['duration']) <= 60
    frames = [f for f in probe['frames'] if f['media_type'] == 'video']
    samples = [f for f in probe['frames'] if f['media_type'] == 'audio']
    assert 0 < len(frames) <= 900
    assert all(a['codec_name'] == 'aac' and a['sample_rate'] == '48000' and a['channels'] == 1 for a in audio)
    decode = execute('ffmpeg', ['-hide_banner', '-nostdin', '-v', 'error', '-xerror', '-threads', '1', '-filter_threads', '1',
                                '-i', '/copies/' + path.name, '-map', '0:v:0', '-map', '0:a:0?', '-fps_mode', 'passthrough', '-f', 'null', '-'])
    row = {'file': path.name, 'bytes': path.stat().st_size, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
           'geometry': [v['width'], v['height']], 'videoDuration': float(v['duration']), 'duration': float(probe['format']['duration']),
           'frames': len(frames), 'audioSamples': sum(f.get('nb_samples', 0) for f in samples),
           'audioDuration': float(audio[0]['duration']) if audio else 0, 'decodeExit': 0}
    native_name = path.name.replace('browser-', 'native-')
    native = args.fixtures / native_name
    if native.exists():
        native_probe = json.loads(execute('ffprobe', ['-v', 'error', '-show_streams', '-show_format', '-of', 'json', '/fixtures/' + native.name]))
        native_video = next(s for s in native_probe['streams'] if s['codec_type'] == 'video')
        assert native_video['width'] == v['width'] and native_video['height'] == v['height']
        assert abs(float(native_video['duration']) - float(v['duration'])) <= 1/15 + .0001
        pixels = []
        for prefix, file in [('/copies/', path.name), ('/fixtures/', native.name)]:
            pixels.append(execute('ffmpeg', ['-hide_banner', '-nostdin', '-v', 'error', '-threads', '1', '-i', prefix + file,
                                             '-frames:v', '1', '-an', '-pix_fmt', 'rgb24', '-f', 'rawvideo', '-']))
        assert len(pixels[0]) == len(pixels[1])
        row['nativeReference'] = {'file': native.name, 'sha256': hashlib.sha256(native.read_bytes()).hexdigest(),
                                   'firstFrameMeanAbsoluteRGBError': sum(abs(a-b) for a, b in zip(*pixels)) / len(pixels[0])}
        if 'vfr' not in native.name:
            assert row['nativeReference']['firstFrameMeanAbsoluteRGBError'] <= 5
    results.append(row)

report = {'version': 1, 'image': args.image, 'sourceCommit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
          'commands': commands, 'results': results, 'qualification': False,
          'limits': 'Synthetic codec/coverage/image comparison only; semantic/human/budget/device gates remain closed.'}
(args.output / 'report.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps({'passed': True, 'copies': len(results), 'image': args.image, 'qualification': False}))
