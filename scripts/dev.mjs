// Start the complete local CPU media stack; seed writes only while both clients
// of its durable jobs are stopped. Planning is pure so tests never wipe a DB.
import { parseArgs } from 'node:util'
import { randomBytes } from 'node:crypto'
import { chmodSync, existsSync, lstatSync, readFileSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { note, repoRoot, run, section } from './lib.mjs'

export const mediaDevEnv = '.env.media.dev'
export function prepareDevEnvironment(root = repoRoot) {
  const path = resolve(root, mediaDevEnv)
  if (!existsSync(path)) {
    const token = randomBytes(32).toString('base64url')
    const contents = [
      'MEDIA_API_URL=http://backend:9000',
      'MEDIA_WORKER_ID=dev-cpu',
      `MEDIA_WORKER_TOKEN=${token}`,
      `MEDIA_WORKER_CREDENTIALS=${JSON.stringify({ 'dev-cpu': token })}`,
      '',
    ].join('\n')
    try { writeFileSync(path, contents, { flag: 'wx', mode: 0o600 }) }
    catch (error) { if (error.code !== 'EEXIST') throw error }
  }
  if (!lstatSync(path).isFile()) throw new Error(`${mediaDevEnv} must be a regular local file`)
  const text = readFileSync(path, 'utf8')
  const fields = Object.fromEntries(text.trim().split('\n').map(line => {
    const at = line.indexOf('=')
    return [line.slice(0, at), line.slice(at + 1)]
  }))
  let credentials
  try { credentials = JSON.parse(fields.MEDIA_WORKER_CREDENTIALS) } catch { /* named error below */ }
  const token = fields.MEDIA_WORKER_TOKEN ?? ''
  const bytes = Buffer.from(token, 'base64url')
  const expected = ['MEDIA_API_URL', 'MEDIA_WORKER_CREDENTIALS', 'MEDIA_WORKER_ID', 'MEDIA_WORKER_TOKEN']
  if (Object.keys(fields).sort().join(',') !== expected.join(',') || bytes.length !== 32 || new Set(bytes).size < 16)
    throw new Error(`${mediaDevEnv} must contain only the generated local media settings`)
  if (fields.MEDIA_WORKER_ID !== 'dev-cpu' || fields.MEDIA_API_URL !== 'http://backend:9000' || !/^[\w-]{43}$/.test(token) || credentials?.['dev-cpu'] !== token || Object.keys(credentials).length !== 1)
    throw new Error(`${mediaDevEnv} is not a matching local API/worker credential file`)
  chmodSync(path, 0o600)
  return path
}

export function devPlan({ seed = false, purgeObjects = false, apiOnly = false } = {}) {
  if (purgeObjects && !seed) throw new Error('--purge-objects requires --seed')
  const compose = ['compose', '--profile', 'dev']
  const steps = []
  if (seed) {
    steps.push(['docker', [...compose, 'stop', 'backend', 'media-worker']])
    if (purgeObjects) {
      steps.push(['docker', [...compose, 'up', '-d', 'minio']])
      steps.push(['docker', [...compose, 'run', '--rm', '--no-deps', '-T', '--entrypoint', 'sh', 'minio-init', '-c',
        'mc alias set local http://minio:9000 postpilot postpilot-dev-secret >/dev/null && if mc ls local/postpilot >/dev/null 2>&1; then mc rm --recursive --force local/postpilot; fi']])
    }
    steps.push(['docker', [...compose, 'run', '--rm', '--no-deps', '-T', 'backend', 'go', 'run', './cmd/seed']])
  }
  steps.push(apiOnly
    ? ['docker', [...compose, 'up', '--build', '--watch', 'backend', 'media-worker']]
    : ['pnpm', ['exec', 'concurrently', '-n', 'web,services', '-c', 'cyan,magenta', 'pnpm run dev:web', 'pnpm run dev:api']])
  return steps
}

export function executeDevPlan(steps, execute = run) {
  for (const [command, args] of steps) execute(command, args)
}

function main() {
  const { values } = parseArgs({ options: {
    seed: { type: 'boolean', default: false },
    'purge-objects': { type: 'boolean', default: false },
    'api-only': { type: 'boolean', default: false },
  }, allowPositionals: false, strict: false })
  const plan = devPlan({ seed: values.seed, purgeObjects: values['purge-objects'], apiOnly: values['api-only'] })
  prepareDevEnvironment()
  section(values.seed ? 'seed + dev' : 'dev')
  note('web → http://localhost:2564 · api → http://localhost:7678 · CPU media worker → private network')
  executeDevPlan(plan)
}
if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) main()
