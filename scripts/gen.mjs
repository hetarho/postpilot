// Code generation: proto → Go·TS (buf) and SQL → Go (sqlc), via Docker.
//
//   pnpm gen          everything that is configured
//   pnpm gen:proto    buf only
//   pnpm gen:sql      sqlc only
//
// If a tool's config isn't present yet, the matching step skips with a note
// instead of failing.

import { run, mount, dockerUser, hasBufConfig, hasSqlcConfig, section, ok, note } from './lib.mjs'

const target = process.argv[2] // undefined | 'proto' | 'sql'
const wantProto = !target || target === 'proto'
const wantSql = !target || target === 'sql'

section('codegen')
let did = false

if (wantProto) {
  if (hasBufConfig()) {
    note('buf generate (proto → Go·TS)')
    // Template lives at backend/buf.gen.yaml, module input is proto/ — both must be
    // explicit (a bare `buf generate` at the repo root has no buf.gen.yaml and fails).
    run('docker', [
      'run', '--rm', ...dockerUser(),
      // buf writes a module cache under $HOME; the image's default home is not
      // writable by an arbitrary --user, so point it at the container's tmpfs.
      '-e', 'HOME=/tmp',
      '-v', mount('', '/work'), '-w', '/work',
      'bufbuild/buf:latest',
      'generate', '--template', 'backend/buf.gen.yaml', 'proto',
    ])
    ok('buf 완료')
    did = true
  } else {
    note('buf 건너뜀 — proto 계약(backend/buf.gen.yaml)이 아직 없음')
  }
}

if (wantSql) {
  if (hasSqlcConfig()) {
    note('sqlc generate (SQL → Go)')
    // sqlc.yaml paths are backend-relative, so backend/ is the container's workdir.
    // The schema it reads is internal/platform/db/migrations — the same files goose
    // applies at boot, so generated types cannot drift from the live schema.
    run('docker', [
      'run', '--rm', ...dockerUser(),
      '-v', mount('backend', '/src'), '-w', '/src',
      'sqlc/sqlc:latest',
      'generate',
    ])
    ok('sqlc 완료')
    did = true
  } else {
    note('sqlc 건너뜀 — backend/sqlc.yaml 이 아직 없음')
  }
}

if (!did) note('아직 생성할 대상 없음. 설정 추가 후 다시 실행하면 자동으로 켜져요.')
