// `pnpm dev` — the host Vite client beside the Air-supervised Compose API (ARCH-42),
// with an optional `--seed` that replaces the local installation's account data first.
//
// Why this is a script rather than the one-line `concurrently` invocation it used to be:
// pnpm appends a script's extra arguments to the end of its command line, so
// `pnpm dev --seed` would have handed `--seed` to concurrently as if it were a third
// command to run. A flag that changes what happens BEFORE the two processes start needs
// somewhere to be read, and this is it.
//
// The seed runs with the API stopped, on purpose. SQLite gives one writer, the api holds
// it for as long as it is up, and a seed racing a live boot is how you get a half-wiped
// database and a 502 nobody can explain.

import { parseArgs } from 'node:util'

import { note, ok, run, section } from './lib.mjs'

const { values } = parseArgs({
  options: {
    seed: { type: 'boolean', default: false },
    // Object storage is left alone by default. A seed run means "give me the fixture", not
    // "throw away the photos I uploaded five minutes ago", and the rows that named them are
    // gone either way — so purging the bucket is asked for explicitly.
    'purge-objects': { type: 'boolean', default: false },
  },
  allowPositionals: false,
  strict: false,
})

if (values.seed) seed({ purgeObjects: Boolean(values['purge-objects']) })

section('dev')
note('web → http://localhost:2564 · api → http://localhost:7678')
// Exactly the command this script replaced, so the everyday path behaves as it always has.
run('pnpm', [
  'exec',
  'concurrently',
  '-n',
  'web,api',
  '-c',
  'cyan,magenta',
  'pnpm run dev:web',
  'pnpm run dev:api',
])

/**
 * Replace the local installation's account data with the fixed test fixture.
 *
 * The api is stopped first and never restarted here: the `concurrently` call above brings
 * it back with the rest of the stack, so a failed seed leaves nothing running and nothing
 * half-written.
 */
function seed({ purgeObjects }) {
  section('seed')

  note('api 중지 (SQLite 쓰기 핸들을 놓아주기 위해)')
  run('docker', ['compose', '--profile', 'dev', 'stop', 'backend'])

  if (purgeObjects) purge()

  note('테스트 계정 데이터 재생성')
  // --no-deps because the seed touches only the database: it needs no bucket, and waiting
  // for MinIO's healthcheck would add half a minute to every reset for nothing.
  run('docker', [
    'compose',
    '--profile',
    'dev',
    'run',
    '--rm',
    '--no-deps',
    '-T',
    'backend',
    'go',
    'run',
    './cmd/seed',
  ])
  ok('시드 완료')
}

/**
 * Empty the local MinIO bucket.
 *
 * Only reachable behind `--purge-objects`. The seeded posts carry no attachments, so what
 * this removes is whatever earlier runs uploaded — objects whose rows the wipe already
 * deleted, and which nothing can reach again.
 *
 * It deliberately never creates the bucket. minio-init creates it AND makes it private,
 * which is the property a presigned-URL-only photo depends on (PRD F-5); a cleanup step
 * that could bring one into existence without the second half of that pair would be a
 * quiet way to publish everything uploaded afterwards. A missing bucket is simply nothing
 * to purge.
 */
function purge() {
  note('MinIO 버킷 비우기')
  run('docker', ['compose', '--profile', 'dev', 'up', '-d', 'minio'])
  run('docker', [
    'compose',
    '--profile',
    'dev',
    'run',
    '--rm',
    '--no-deps',
    '-T',
    '--entrypoint',
    'sh',
    'minio-init',
    '-c',
    'mc alias set local http://minio:9000 postpilot postpilot-dev-secret >/dev/null && ' +
      'if mc ls local/postpilot >/dev/null 2>&1; then mc rm --recursive --force local/postpilot; fi',
  ])
  ok('오브젝트 정리 완료')
}
