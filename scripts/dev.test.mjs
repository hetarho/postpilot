import { test } from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { devPlan, executeDevPlan, mediaDevEnv, prepareDevEnvironment } from './dev.mjs'
import { checkMediaAssetLabel } from './media-assets.mjs'

test('API development explicitly rebuilds/watches both media services', () => {
  assert.deepEqual(devPlan({ apiOnly: true }), [['docker', ['compose', '--profile', 'dev', 'up', '--build', '--watch', 'backend', 'media-worker']]])
  assert.ok(devPlan()[0][1].includes('pnpm run dev:api'))
})

test('a failed seed leaves both API and worker stopped and never purges implicitly', () => {
  const calls = []
  assert.throws(() => executeDevPlan(devPlan({ seed: true }), (cmd, args) => {
    calls.push([cmd, args])
    if (args.includes('./cmd/seed')) throw new Error('seed failed')
  }), /seed failed/)
  assert.deepEqual(calls[0][1].slice(-3), ['stop', 'backend', 'media-worker'])
  assert.equal(calls.length, 2)
  assert.ok(calls.every(([, args]) => !args.includes('up') && !args.includes('minio-init')))
})

test('successful seed starts only after data replacement, and purge requires its flag', () => {
  const calls = []
  executeDevPlan(devPlan({ seed: true, purgeObjects: true, apiOnly: true }), (cmd, args) => calls.push([cmd, args]))
  assert.deepEqual(calls[0][1].slice(-3), ['stop', 'backend', 'media-worker'])
  assert.ok(calls[2][1].includes('minio-init'))
  assert.ok(calls[3][1].includes('./cmd/seed'))
  assert.deepEqual(calls[4][1].slice(-2), ['backend', 'media-worker'])
  assert.throws(() => devPlan({ purgeObjects: true }), /requires --seed/)
})

test('fresh dev credentials are private, unique and stable across restarts', () => {
  const a = mkdtempSync(join(tmpdir(), 'postpilot-dev-a-'))
  const b = mkdtempSync(join(tmpdir(), 'postpilot-dev-b-'))
  try {
    const first = prepareDevEnvironment(a)
    const original = readFileSync(first, 'utf8')
    assert.equal(prepareDevEnvironment(a), first)
    assert.equal(readFileSync(first, 'utf8'), original)
    assert.notEqual(readFileSync(prepareDevEnvironment(b), 'utf8'), original)
    if (process.platform !== 'win32') assert.equal(statSync(first).mode & 0o777, 0o600)
    assert.doesNotMatch(original, /R2_|DB_PATH|PROVIDERS|OPENAI/)
    writeFileSync(join(a, mediaDevEnv), original + 'DB_PATH=/private/database\n')
    assert.throws(() => prepareDevEnvironment(a), /only the generated/)
  } finally { rmSync(a, { recursive: true, force: true }); rmSync(b, { recursive: true, force: true }) }
})

test('the CPU image label matches the pinned media asset inputs', () => checkMediaAssetLabel())
