import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { spawnSync } from 'node:child_process'
import { cpSync, existsSync, mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { basename, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const build = fileURLToPath(new URL('../backend/build/', import.meta.url))
const packages = ['zlib', 'zlib-dev', 'zlib-static']
const filename = (name) => `${name}-1.3.2-r1.apk`
const artifacts = join(build, 'zlib-apks', 'x86_64')
const source = readFileSync(join(build, 'media-tools.sh'), 'utf8')
const boundary = '\nmkdir -p /media-build /opt/media/share/licenses /opt/media/share/sources\n'
assert.equal(source.split(boundary).length, 2, 'exercise the complete install prefix before compilation')

function install({ indexed = false, retained = true, corrupt, missing, reject = false, networkDown = false, arch = 'x86_64' } = {}) {
  const root = mkdtempSync(join(tmpdir(), 'postpilot signed APKs '))
  try {
    const bin = join(root, 'bin')
    mkdirSync(bin)
    const script = join(root, 'media-tools.sh')
    writeFileSync(script, source.split(boundary)[0] + '\n')
    if (retained) {
      const retainedDir = join(root, 'zlib-apks', 'x86_64')
      cpSync(artifacts, retainedDir, { recursive: true })
      if (corrupt) writeFileSync(join(retainedDir, filename(corrupt)), 'corrupt package')
      if (missing) rmSync(join(retainedDir, filename(missing)))
    }
    const log = join(root, 'calls.jsonl')
    const stub = `#!${process.execPath}
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const args = process.argv.slice(2);
const command = path.basename(process.argv[1]);
fs.appendFileSync(process.env.CALL_LOG, JSON.stringify({ command, args }) + '\\n');
if (command === 'apk') {
  if (args[0] === '--print-arch') { console.log(process.env.TEST_ARCH); process.exit(0); }
  const local = args.filter(arg => arg.endsWith('.apk'));
  process.exit(local.length ? (process.env.REJECT_APK === 'true' ? 1 : 0) : (process.env.INDEX_OK === 'true' ? 0 : 1));
}
if (command === 'wget') {
  if (process.env.NETWORK_DOWN === 'true') process.exit(1);
  fs.copyFileSync(path.join(process.env.ARTIFACTS, path.basename(args.at(-1))), args[args.indexOf('-O') + 1]);
}
if (command === 'sha256sum') {
  for (const line of fs.readFileSync(0, 'utf8').trim().split('\\n')) {
    const [, expected, file] = /^(\\S+)\\s+(.+)$/.exec(line);
    if (crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex') !== expected) process.exit(1);
  }
}
`
    for (const command of ['apk', 'wget', 'sha256sum']) {
      writeFileSync(join(bin, command), stub, { mode: 0o755 })
    }
    const result = spawnSync('sh', [script], {
      encoding: 'utf8',
      env: { ...process.env, PATH: `${bin}:${process.env.PATH}`, CALL_LOG: log, ARTIFACTS: artifacts, INDEX_OK: String(indexed), REJECT_APK: String(reject), NETWORK_DOWN: String(networkDown), TEST_ARCH: arch },
    })
    assert.ifError(result.error)
    const calls = existsSync(log) ? readFileSync(log, 'utf8').trim().split('\n').map(line => JSON.parse(line)) : []
    const apkCalls = calls.filter(call => call.command === 'apk' && call.args[0] === 'add')
    const downloads = calls.filter(call => call.command === 'wget')
    // The actual shell trap must reclaim the attempt directory even on failure.
    for (const call of apkCalls) {
      for (const path of call.args.filter(arg => arg.endsWith('.apk'))) assert.equal(existsSync(path), false)
    }
    return { ...result, apkCalls, downloads, calls }
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
}

test('retained x86_64 APKs match the already qualified signed-package hashes', () => {
  const hashes = [
    '63aeea03c15a2f9018f81805cfc8aa926bdf5cd68921f22149c2fbb5d0ee9f47',
    '50e47b778d894841a9c8ace7640edc1e585269963f29d6e4fdc008c8d6364890',
    '1b7ce8d0515576599450144a5652eeba0e04e4b7342bba32cbc5a2a4ae256939',
  ]
  packages.forEach((name, index) => assert.equal(createHash('sha256').update(readFileSync(join(artifacts, filename(name)))).digest('hex'), hashes[index]))
})

test('an indexed success never consults even a damaged retained APK', () => {
  const result = install({ indexed: true, corrupt: 'zlib-static' })
  assert.equal(result.status, 0, result.stderr)
  assert.equal(result.apkCalls.length, 1)
  assert.equal(result.calls.length, 1)
})

test('retained packages recover an index rotation with every APK download unavailable', () => {
  const result = install({ networkDown: true })
  assert.equal(result.status, 0, result.stderr)
  assert.equal(result.downloads.length, 0)
  assert.equal(result.apkCalls.length, 2)
  assert.deepEqual(result.apkCalls[1].args.filter(arg => arg.endsWith('.apk')).map(path => basename(path)), packages.map(filename))
  assert.ok(result.calls.find(call => call.command === 'sha256sum'))
})

test('a missing retained package alone uses the existing bounded network fallback', () => {
  const result = install({ missing: 'zlib-static' })
  assert.equal(result.status, 0, result.stderr)
  assert.equal(result.downloads.length, 1)
  assert.equal(basename(result.downloads[0].args.at(-1)), filename('zlib-static'))
  assert.deepEqual(result.downloads[0].args.slice(0, 3), ['-q', '-T', '20'])
})

test('an unbundled build retains the three original download paths', () => {
  const result = install({ retained: false })
  assert.equal(result.status, 0, result.stderr)
  assert.equal(result.downloads.length, 3)
})

for (const corrupt of packages) {
  test(`a damaged retained ${corrupt} stops before APK installation or network recovery`, () => {
    const result = install({ corrupt })
    assert.notEqual(result.status, 0)
    assert.match(result.stderr, /checksum verification failed/)
    assert.equal(result.apkCalls.length, 1)
    assert.equal(result.downloads.length, 0)
  })
}

test('a signature or resolver refusal remains a failure after retained hash verification', () => {
  const result = install({ reject: true })
  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /signature verification or installation failed/)
  assert.equal(result.apkCalls.length, 2)
  assert.equal(result.downloads.length, 0)
})

test('a missing package and unavailable mirror fail without admitting a partial install', () => {
  const result = install({ missing: 'zlib-static', networkDown: true })
  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /Unable to download the pinned zlib-static/)
  assert.equal(result.apkCalls.length, 1)
})

test('an unqualified architecture never consumes the retained x86_64 packages', () => {
  const result = install({ arch: 'unknown' })
  assert.notEqual(result.status, 0)
  assert.equal(result.downloads.length, 0)
  assert.equal(result.apkCalls.length, 1)
})
