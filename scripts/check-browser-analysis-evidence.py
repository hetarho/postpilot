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
correction = json.loads((qa / 'browser-analysis-preparation-eof-correction.json').read_text())
for relative, expected in checks['pipelineSHA256'].items():
    assert hashlib.sha256((root / relative).read_bytes()).hexdigest() == expected, relative
review = checks['reviewCorrections']
assert correction['at'] == review['at'] and correction['semanticQualified'] is False
for relative, expected in review['sourceSHA256'].items():
    assert hashlib.sha256((root / relative).read_bytes()).hexdigest() == expected, relative
assert review['sourceSHA256'] == correction['sourceSHA256']
reuse = review['nativeCodecReuse']
copy_path = 'frontend/src/shared/lib/media/analysis-copy.ts'
marker = reuse['marker'].encode()
current_source = (root / copy_path).read_bytes()
previous_source = subprocess.check_output(['git', 'show', review['reviewedHead'] + ':' + copy_path], cwd=root)
current = current_source[current_source.index(marker):]
previous = previous_source[previous_source.index(marker):]
assert current == previous and hashlib.sha256(current).hexdigest() == reuse['unchangedEncodingMuxInspectionAndRetrySuffixSHA256']
for relative, expected in checks['pipelineSHA256'].items():
    if relative != copy_path:
        prior = subprocess.check_output(['git', 'show', review['reviewedHead'] + ':' + relative], cwd=root)
        assert hashlib.sha256(prior).hexdigest() == expected, relative
assert reuse == correction['nativeReuse'] and reuse['actualNativeCommands'] == 47 and reuse['actualNativeArtifacts'] == 10
sparse = correction['actualSparseEOF']
assert sparse['qualification'] is False and len(sparse['results']) == 3
sparse_rows = {row['id']: row for row in sparse['results']}
for name in ['span-short', 'span-missing']:
    assert sparse_rows[name]['error'] == 'CLIP_INPUT_TOO_LARGE', name
valid = sparse_rows['span-valid']['original']
assert valid['provenance'] == 'browser_client' and valid['durationMs'] == 6000 and valid['decodedFrames'] == 3 and valid['cadenceVerified'] is False
originals = correction['actualNormalOriginalEOF']
assert originals['qualification'] is False and len(originals['results']) == 9
historical = {row['id']: row for row in client['results']}
fields = review['normalOriginalMeasurementEquality']['fields']
for row in originals['results']:
    assert row['original']['provenance'] == 'browser_client'
    assert originals['fixtures'][row['id']] == client['fixtures'][row['id']], row['id']
    for field in fields:
        assert row['original'][field] == historical[row['id']]['original'][field], (row['id'], field)
    for event in row['resourceEvents']:
        assert event['resources']['liveDecodedFrames'] == 0 and event['resources']['liveAudioData'] == 0
        assert event['resources']['peakDecodedFrames'] <= 48
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
print('Exact corrected ownership/EOF source, 3 sparse and 9 unchanged original measurements, reused codec/artifact receipts, 27 deploy inputs and 218/152 generator hashes verified; semantic/device gates remain closed.')
