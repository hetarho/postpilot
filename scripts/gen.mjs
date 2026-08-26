// Code generation: proto → Go·TS (buf), via Docker.
//
//   pnpm gen          everything that is configured
//   pnpm gen:proto    buf only
//
// If a tool's config isn't present yet, the matching step skips with a note
// instead of failing.

import { run, mount, hasBufConfig, section, ok, note } from './lib.mjs'

const target = process.argv[2] // undefined | 'proto'
const wantProto = !target || target === 'proto'

section('codegen')
let did = false

if (wantProto) {
  if (hasBufConfig()) {
    note('buf generate (proto → Go·TS)')
    // Template lives at backend/buf.gen.yaml, module input is proto/ — both must be
    // explicit (a bare `buf generate` at the repo root has no buf.gen.yaml and fails).
    run('docker', [
      'run', '--rm',
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

if (!did) note('아직 생성할 대상 없음. 설정 추가 후 다시 실행하면 자동으로 켜져요.')
