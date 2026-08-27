#!/usr/bin/env node
// Scaffold the next change proposal (targets an existing plan).
// Usage: node new-change.mjs <planNN> "<title>"  (pnpm spec:change <planNN> "<title>")
// /create-change interviews the user, then fills as-is→to-be · scope · acceptance criteria (WHAT delta). STEPS live in the job.
import { readFileSync, writeFileSync, readdirSync, existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve, join } from 'node:path'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const planDir = join(root, 'spec', 'plan')
const changesDir = join(root, 'spec', 'changes')
const tpl = join(root, 'scripts', 'templates', 'change.md')

const args = process.argv.slice(2).filter((a) => !a.startsWith('--'))
const planArg = args[0]
const title = args.slice(1).join(' ').trim()
if (!planArg || !title) {
  console.error(
    'Usage: pnpm spec:change <planNN> "<title>"  (e.g. pnpm spec:change 15 "manage themes in DB")',
  )
  process.exit(1)
}

const pnn = String(planArg)
  .replace(/[^0-9]/g, '')
  .padStart(2, '0')
const planFile = readdirSync(planDir).find((f) => f.startsWith(pnn + '.') && f.endsWith('.md'))
if (!planFile) {
  console.error(`No plan at spec/plan/${pnn}.*.md (change targets an existing plan)`)
  process.exit(1)
}
const planRef = `plan/${planFile.replace(/\.md$/, '')}`

const nn = nextNum(changesDir)
const out = join(changesDir, `${nn}.${slugify(title)}.md`)
if (existsSync(out)) {
  console.error(`${out} exists`)
  process.exit(1)
}
writeFileSync(out, fill(readFileSync(tpl, 'utf8'), { NN: nn, TITLE: title, PLAN: planRef }), 'utf8')
console.log(`Created spec/changes/${nn}.${slugify(title)}.md  (targets ${planRef})`)
console.log(
  'Next: /create-change fills as-is→to-be · scope · acceptance criteria via interview, then /create-change-job ' +
    nn +
    '.',
)

// Monotonic across live + archive/: completed proposals move to archive/ but their numbers must never be reused
// (jobs reference changes by number via frontmatter `source`). Scanning only the live dir re-issues numbers.
function nextNum(dir) {
  const archive = join(dir, 'archive')
  const files = existsSync(archive)
    ? [...readdirSync(dir), ...readdirSync(archive)]
    : readdirSync(dir)
  const m = files.map((f) => parseInt((f.match(/^(\d+)/) || [])[1], 10)).filter((n) => !isNaN(n))
  return String((m.length ? Math.max(...m) : 0) + 1).padStart(2, '0')
}
function slugify(s) {
  return (
    s
      .toLowerCase()
      .replace(/[\\/:*?"<>|.]+/g, '')
      .replace(/\s+/g, '-')
      .replace(/-+/g, '-')
      .replace(/^-|-$/g, '')
      .slice(0, 40) || 'untitled'
  )
}
function fill(t, m) {
  return Object.entries(m).reduce((a, [k, v]) => a.replaceAll(`{{${k}}}`, v), t)
}
