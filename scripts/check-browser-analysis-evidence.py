"""Check exact offline source/receipt/artifact equality without a provider call."""
import hashlib
import json
from pathlib import Path
import sys
import subprocess

root = Path(__file__).resolve().parents[1]
qa = root / 'docs/qa'
checks = json.loads((qa / 'browser-analysis-preparation-checks.json').read_text())
client = json.loads((qa / 'browser-analysis-preparation-browser.json').read_text())
native = json.loads((qa / 'browser-analysis-preparation-native.json').read_text())
for relative, expected in checks['pipelineSHA256'].items():
    assert hashlib.sha256((root / relative).read_bytes()).hexdigest() == expected, relative
receipt = checks['deployReuse']['receipt']
assert receipt['exit'] == 0 and checks['deployReuse']['matched'] == 27
for relative, expected in receipt['inputHashes'].items():
    assert hashlib.sha256((root / relative).read_bytes()).hexdigest() == expected, relative
parent = checks['parentReuse']
for part, expected in parent['exactTrees'].items():
    assert subprocess.check_output(['git', 'rev-parse', 'HEAD:' + part], cwd=root, text=True).strip() == expected, part
for kind in ['generatorInputHashes', 'generatorOutputHashes']:
    for relative, expected in parent[kind].items():
        assert hashlib.sha256((root / relative).read_bytes()).hexdigest() == expected, relative
assert parent['generatorInputs'] == 218 and parent['generatorOutputs'] == 152
assert client['qualification'] is False and native['qualification'] is False
assert len(client['results']) == 12 and len(native['results']) == 10
assert len(native['commands']) == 47 and all(command['exit'] == 0 for command in native['commands'])
assert client['rangeSummary']['largestRequestedBytes'] <= 4 * 1024 * 1024
for row in client['results']:
    if 'original' in row:
        assert row['original']['provenance'] == 'browser_client'
    for event in row.get('resourceEvents', []):
        resources = event['resources']
        assert resources['liveDecodedFrames'] == 0 and resources['liveAudioData'] == 0
        assert resources['peakDecodedFrames'] <= 48
for row in native['results']:
    assert 0 < row['bytes'] <= 8 * 1024 * 1024 and 0 < row['duration'] <= 60
    assert 0 < row['frames'] <= 900 and max(row['geometry']) <= 720
    audio = row.get('nativeReference', {}).get('audio')
    if audio:
        assert .97 <= audio['rmsRatio'] <= 1.03 and audio['correlation'] >= .995
if len(sys.argv) == 2:
    artifacts = Path(sys.argv[1])
    for row in native['results']:
        data = (artifacts / row['file']).read_bytes()
        assert len(data) == row['bytes'] and hashlib.sha256(data).hexdigest() == row['sha256']
print('Exact analysis pipeline/artifact receipts and 27 unchanged deploy inputs plus 218/152 generator hashes verified; semantic/device gates remain closed.')
