// 게이트 이미지 청소.
//
//   pnpm docker:gc                 지울 목록만 보여준다 (기본)
//   pnpm docker:gc --yes           실제로 지운다
//   pnpm docker:gc --hours 72      24시간 대신 72시간 이내 빌드를 남긴다
//   pnpm docker:gc --cache --yes   빌드 캐시까지 비운다 (다음 빌드가 많이 느려진다)
//
// Why this exists: the clip media/render gates only run inside the production
// image, so every verified task leaves one behind. A task-specific tag
// (postpilot-api:t151) keeps its image referenced forever, so `docker image
// prune` never has anything to collect and the pile only grows — 60 images and
// 18 GB by the time anyone looked. Gate builds now reuse the fixed postpilot:gate*
// tags (see README), and this sweeps what those replace plus anything older.
//
// What it will NOT touch: an image a container still references (running or
// stopped), anything built inside the keep window — a parallel session is
// probably still using it — and any repository outside postpilot*/pp-builder.

import { capture, run, section, note, ok, fail } from './lib.mjs'

const argv = process.argv.slice(2)
const apply = argv.includes('--yes') || argv.includes('-y')
const sweepCache = argv.includes('--cache')
const hoursArg = argv.indexOf('--hours')
const keepHours = hoursArg === -1 ? 24 : Number(argv[hoursArg + 1])
if (!Number.isFinite(keepHours) || keepHours < 0) fail('--hours 는 0 이상의 숫자여야 해요.')

// Only the repositories this repo's gates produce. Everything else on the
// machine — other projects, base images — is somebody else's business.
const OURS = /^(postpilot|pp-builder)/

const bytes = (text) => {
  const m = /^([\d.]+)\s*(B|kB|KB|MB|GB)$/.exec(text.trim())
  if (!m) return 0
  return Number(m[1]) * { B: 1, kB: 1e3, KB: 1e3, MB: 1e6, GB: 1e9 }[m[2]]
}
const gb = (n) => `${(n / 1e9).toFixed(2)} GB`

// A container pins its image by whatever reference it was started with, and by
// the resolved ID. Collect both: `{{.Image}}` is a tag for some containers and a
// bare ID for others.
function pinned() {
  const refs = new Set(
    capture('docker', ['ps', '-a', '--format', '{{.Image}}'])
      .split('\n')
      .map((l) => l.trim())
      .filter(Boolean),
  )
  const ids = capture('docker', ['ps', '-aq'])
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)
  if (ids.length > 0) {
    for (const id of capture('docker', ['inspect', '--format', '{{.Image}}', ...ids]).split('\n')) {
      const resolved = id.trim().replace(/^sha256:/, '')
      if (resolved) refs.add(resolved)
    }
  }
  return refs
}

function sweepable(held) {
  const now = Date.now()
  const rows = capture('docker', [
    'images',
    '--format',
    '{{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}\t{{.CreatedAt}}',
  ])
  const out = []
  for (const line of rows.split('\n')) {
    if (!line.trim()) continue
    const [repo, tag, id, size, created] = line.split('\t')
    const dangling = repo === '<none>'
    if (!dangling && !OURS.test(repo)) continue
    const ref = `${repo}:${tag}`
    // Docker prints its own local timestamp with an offset ("… +0900 KST"); the
    // trailing zone name is not something Date parses, so drop it.
    const at = new Date(created.replace(/\s+[A-Z]{2,5}$/, ''))
    const hours = (now - at.getTime()) / 3_600_000
    if (hours < keepHours) continue
    if (held.has(ref) || held.has(repo) || [...held].some((h) => h.startsWith(id))) continue
    out.push({ ref: dangling ? id : ref, id, dangling, bytes: bytes(size), size, hours })
  }
  return out.sort((a, b) => b.bytes - a.bytes)
}

section(`docker 게이트 이미지 청소 (최근 ${keepHours}시간 · 사용 중 이미지는 제외)`)
const held = pinned()
const targets = sweepable(held)
if (targets.length === 0) {
  ok('지울 이미지가 없어요.')
  process.exit(0)
}
for (const t of targets) {
  const days = t.hours >= 48 ? `${Math.round(t.hours / 24)}일 전` : `${Math.round(t.hours)}시간 전`
  note(`${t.ref.padEnd(42)} ${t.size.padStart(8)}  ${days}${t.dangling ? '  (태그 없음)' : ''}`)
}
const total = targets.reduce((sum, t) => sum + t.bytes, 0)
note(`${targets.length}개 · 합계 ${gb(total)} (레이어를 공유하므로 실제 회수는 더 적어요)`)

if (!apply) {
  ok('목록만 보여줬어요. 실제로 지우려면 --yes 를 붙이세요.')
  process.exit(0)
}
// In batches: one `docker rmi` per image would be slow, and one call with every
// reference stops at the first that another image still depends on.
for (let i = 0; i < targets.length; i += 20) {
  run('docker', ['rmi', ...targets.slice(i, i + 20).map((t) => t.ref)])
}
ok(`${targets.length}개 삭제`)

if (sweepCache) {
  note('빌드 캐시를 비웁니다 — 다음 이미지 빌드는 ffmpeg/resvg 소스 빌드부터 다시 돌아요.')
  run('docker', ['builder', 'prune', '-f'])
}
run('docker', ['system', 'df'])
