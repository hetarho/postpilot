import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'

const root = fileURLToPath(new URL('..', import.meta.url))
const failures = []

function repoPath(relative) {
  return path.join(root, relative)
}

function read(relative) {
  return readFileSync(repoPath(relative), 'utf8')
}

function walk(relative, accept) {
  const absolute = repoPath(relative)
  if (!existsSync(absolute)) return []
  const result = []
  for (const entry of readdirSync(absolute, { withFileTypes: true })) {
    const child = path.posix.join(relative, entry.name)
    if (entry.isDirectory()) {
      result.push(...walk(child, accept))
    } else if (entry.isFile() && accept(child)) {
      result.push(child)
    }
  }
  return result
}

function rejectMatches(files, patterns, label) {
  for (const file of files) {
    const source = read(file)
    for (const pattern of patterns) {
      pattern.lastIndex = 0
      if (pattern.test(source)) failures.push(`${label}: ${file} matches ${pattern}`)
    }
  }
}

const forbiddenPaths = [
  'agent',
  'proto/postpilot/v1/publishing.proto',
  'backend/internal/publishing',
  'backend/cmd/retirepublishing',
  'backend/internal/gen/postpilot/v1/publishing.pb.go',
  'backend/internal/gen/postpilot/v1/postpilotv1connect/publishing.connect.go',
  'frontend/src/shared/api/gen/postpilot/v1/publishing_pb.ts',
  'frontend/src/entities/publish-job',
  'frontend/src/entities/publishing-agent',
  'frontend/src/features/publish-post',
  'frontend/src/pages/publishing-agents',
  'frontend/src/widgets/publish-panel',
]
for (const retiredPath of forbiddenPaths) {
  if (existsSync(repoPath(retiredPath))) failures.push(`retired path exists: ${retiredPath}`)
}

const backendRuntime = walk(
  'backend',
  (file) =>
    file.endsWith('.go') &&
    !file.endsWith('_test.go') &&
    !file.includes('/internal/platform/db/migrations/') &&
    !file.includes('/internal/gen/'),
)
const frontendRuntime = walk(
  'frontend/src',
  (file) =>
    /\.[cm]?[tj]sx?$/.test(file) &&
    !/\.(?:test|spec)\.[cm]?[tj]sx?$/.test(file) &&
    !file.includes('/__snapshots__/'),
)
const protoRuntime = walk('proto', (file) => file.endsWith('.proto'))
const buildRuntime = [
  'package.json',
  'scripts/gen.mjs',
  'scripts/dev.mjs',
  'buf.yaml',
  'backend/buf.gen.yaml',
  'frontend/buf.gen.yaml',
  '.github/workflows/ci.yml',
  'backend/Dockerfile',
  'docker-compose.yml',
  'docker-compose.prod.yml',
].filter((file) => existsSync(repoPath(file)))

const retiredContract = [
  /\bPublishingService\b/,
  /\bPublishingAgentService\b/,
  /\bCreateAgentPairing(?:Request|Response)?\b/,
  /\bStartPublish(?:Request|Response)?\b/,
  /\bClaimPublishJob(?:Request|Response)?\b/,
  /\bRenewPublishLease(?:Request|Response)?\b/,
  /\bReportPublishProgress(?:Request|Response)?\b/,
  /\bCompletePublish(?:Request|Response)?\b/,
  /\bFailPublish(?:Request|Response)?\b/,
  /\bSyncAgentProfile(?:Request|Response)?\b/,
]
rejectMatches(
  [...backendRuntime, ...frontendRuntime, ...protoRuntime, ...buildRuntime],
  retiredContract,
  'retired publishing contract',
)

const retiredOwnership = [
  /\bpublish_assets\b/,
  /\bpublish_jobs\b/,
  /\bpublish_job_ids\b/,
  /\bpublishing_agents\b/,
  /\bpublishing_pairings\b/,
  /\bretirepublishing\b/,
]
rejectMatches(
  [...backendRuntime, ...protoRuntime, ...buildRuntime],
  retiredOwnership,
  'retired runtime ownership',
)

const retiredAgentTarget = [
  /["']agent:(?:setup|run|diagnostics|build|install)["']/,
  /agent\/internal\/gen/,
  /working-directory:\s*agent\b/,
  /go-version-file:\s*agent\/go\.mod/,
  /cmd\/postpilot-agent/,
  /github\.com\/postpilot\/agent/,
]
rejectMatches(buildRuntime, retiredAgentTarget, 'retired companion build target')

const legacyRoute = '/publishing-agents'
const legacyOccurrences = []
for (const file of frontendRuntime) {
  const source = read(file)
  let index = source.indexOf(legacyRoute)
  while (index !== -1) {
    legacyOccurrences.push(file)
    index = source.indexOf(legacyRoute, index + legacyRoute.length)
  }
}
if (
  legacyOccurrences.length !== 1 ||
  legacyOccurrences[0] !== 'frontend/src/app/routes/publishing.ts'
) {
  failures.push(`legacy route exceptions changed: ${legacyOccurrences.join(', ') || 'none'}`)
}
const legacyRouteSource = read('frontend/src/app/routes/publishing.ts')
if (!legacyRouteSource.includes("redirect({ to: '/posts', replace: true })")) {
  failures.push('legacy publishing route no longer redirects directly to /posts')
}
rejectMatches(
  ['frontend/src/app/routes/publishing.ts'],
  [/\b(?:queryOptions|mutationOptions|connectQuery|createClient)\b/],
  'legacy redirect performs runtime work',
)

rejectMatches(
  ['README.md'],
  [/Mac 발행 에이전트/, /agent:install/, /postpilot-agent\s+(?:setup|run|install)/, /네이버에 발행/],
  'active README publishing promise',
)
rejectMatches(
  ['spec/NARRATIVE.md'],
  [
    /운영자 등급 사용자는[^\n]*페어링된 Mac/,
    /`네이버에 발행`/,
    /자동 네이버 발행은 이 복사 기능의 대체가 아니라 별도 경로/,
    /T003[^\n]*T008[^\n]*발행/,
  ],
  'derived narrative publishing promise',
)

const deploy = read('DEPLOY.md')
// Rollout status changes as environments are retired. Keep requiring the bridge and
// evidence-bearing cleanup procedure, without freezing the runbook in its pending state.
for (const required of [
  'T312 bridge 이미지',
  'migration 0072',
  'migration 0076',
  '--report-digest',
  '--shutdown-inventory',
  '--shutdown-digest',
  '--receipt',
  '--verify',
]) {
  if (!deploy.includes(required)) failures.push(`DEPLOY retirement bridge lost: ${required}`)
}

if (failures.length > 0) {
  process.stderr.write(`Publishing retirement absence check failed:\n${failures.map((failure) => `- ${failure}`).join('\n')}\n`)
  process.exitCode = 1
} else {
  process.stdout.write(
    'Publishing retirement absence check passed. Reviewed exceptions: migrations 0010/0015/0037/0072/0076, DB regression assertions, reserved FailureReason identities, DEPLOY bridge instructions, and the single authenticated legacy redirect.\n',
  )
}
