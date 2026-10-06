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
    (args.output / 'commands.json').write_text(json.dumps({'image': args.image, 'commands': commands}, indent=2) + '\n')
    if p.returncode != 0:
        raise RuntimeError(p.stderr.decode())
    if len(p.stdout) > 16 * 1024 * 1024:
        raise RuntimeError('Diagnostic output exceeds bound')
    return p.stdout


def png_rgb(data):
    import struct, zlib
    assert data[:8] == b'\x89PNG\r\n\x1a\n'
    offset, encoded = 8, bytearray()
    while offset < len(data):
        length = int.from_bytes(data[offset:offset+4], 'big'); kind = data[offset+4:offset+8]; body = data[offset+8:offset+8+length]; offset += length+12
        if kind == b'IHDR':
            width, height, depth, color, compression, filtering, interlace = struct.unpack('>IIBBBBB', body)
            assert depth == 8 and color in (2, 6) and interlace == 0 and width * height <= 720 * 720
        if kind == b'IDAT': encoded.extend(body)
    channels = 3 if color == 2 else 4; stride = width * channels
    raw = zlib.decompress(encoded); assert len(raw) == height * (stride + 1)
    result, previous, cursor = bytearray(), bytearray(stride), 0
    def paeth(a,b,c):
        p = a+b-c; distances = [abs(p-a),abs(p-b),abs(p-c)]
        return [a,b,c][distances.index(min(distances))]
    for _ in range(height):
        kind = raw[cursor]; cursor += 1; row = bytearray(raw[cursor:cursor+stride]); cursor += stride
        for i in range(stride):
            a = row[i-channels] if i >= channels else 0; b = previous[i]; c = previous[i-channels] if i >= channels else 0
            predictor = 0 if kind == 0 else a if kind == 1 else b if kind == 2 else (a+b)//2 if kind == 3 else paeth(a,b,c) if kind == 4 else None
            assert predictor is not None; row[i] = (row[i] + predictor) & 255
        if channels == 3: result.extend(row)
        else:
            for i in range(0, stride, channels): result.extend(row[i:i+3])
        previous = row
    return result


browser_report = json.loads((args.copies / 'report.json').read_text())
expected = {f"browser-{row['id']}-{copy['slot']['ordinal']}.mp4": copy['slot'] for row in browser_report['results'] for copy in row.get('copies', [])}
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
    slot = expected[path.name]
    assert 0 < len(frames) <= 900
    assert [v['width'], v['height']] == [slot['width'], slot['height']] and bool(audio) == slot['hasAudio']
    assert abs(float(v['duration']) * 1000 - slot['durationMs']) <= 1000/15 + .01
    assert all(f['width'] == v['width'] and f['height'] == v['height'] and f['pix_fmt'] == 'yuv420p' and f['sample_aspect_ratio'] == '1:1' for f in frames)
    assert all(abs(float(f['best_effort_timestamp_time']) - index/15) < .000002 for index, f in enumerate(frames))
    assert sum(f.get('nb_samples', 0) for f in samples) <= slot['durationMs'] * 48 + 1024
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
            pixels.append(png_rgb(execute('ffmpeg', ['-hide_banner', '-nostdin', '-v', 'error', '-threads', '1', '-i', prefix + file,
                                             '-frames:v', '1', '-an', '-pix_fmt', 'rgb24', '-c:v', 'png', '-f', 'image2pipe', '-'])))
        assert len(pixels[0]) == len(pixels[1])
        row['nativeReference'] = {'file': native.name, 'sha256': hashlib.sha256(native.read_bytes()).hexdigest(),
                                   'firstFrameMeanAbsoluteRGBError': sum(abs(a-b) for a, b in zip(*pixels)) / len(pixels[0])}
        if 'vfr' not in native.name:
            assert row['nativeReference']['firstFrameMeanAbsoluteRGBError'] <= 5
    if audio and native.exists():
        decoded = []
        import array, math
        for prefix, file in [('/copies/', path.name), ('/fixtures/', native.name)]:
            raw = execute('ffmpeg', ['-hide_banner', '-nostdin', '-v', 'error', '-threads', '1', '-i', prefix + file,
                                      '-map', '0:a:0', '-vn', '-ac', '1', '-ar', '48000', '-c:a', 'pcm_s16le', '-f', 's16le', '-'])
            pcm = array.array('h'); pcm.frombytes(raw); decoded.append(pcm)
        trim = min(len(decoded[0]), len(decoded[1]), slot['durationMs'] * 48)
        margin = min(4096, trim//4)
        planes = [pcm[margin:trim-margin] for pcm in decoded]
        rms = [math.sqrt(sum(x*x for x in pcm) / len(pcm)) for pcm in planes]
        denominator = math.sqrt(sum(x*x for x in planes[0]) * sum(x*x for x in planes[1]))
        correlation = sum(x*y for x,y in zip(*planes)) / denominator if denominator else 1
        assert .97 <= rms[0] / rms[1] <= 1.03 if rms[1] else True
        assert correlation >= .995
        row['nativeReference']['audio'] = {'browserSamples': len(decoded[0]), 'nativeSamples': len(decoded[1]), 'rms': rms, 'rmsRatio': rms[0] / rms[1] if rms[1] else 1, 'correlation': correlation}
    results.append(row)
    (args.output / 'results.json').write_text(json.dumps(results, indent=2) + '\n')

report = {'version': 1, 'image': args.image, 'sourceCommit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
          'commands': commands, 'results': results, 'qualification': False,
          'limits': 'Synthetic codec/coverage/image comparison only; semantic/human/budget/device gates remain closed.'}
(args.output / 'report.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps({'passed': True, 'copies': len(results), 'image': args.image, 'qualification': False}))
