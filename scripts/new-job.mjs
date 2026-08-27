#!/usr/bin/env node
// Scaffold the next sequential implementation job from a plan, change, or refactor report.
// Usage:
//   pnpm spec:job plan 15
//   pnpm spec:job change 03
//   pnpm spec:job refactor 01 "Split UniverseCanvas controllers"
import { readFileSync, writeFileSync, readdirSync, existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve, join } from 'node:path'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const planDir = join(root, 'spec', 'plan')
const changesDir = join(root, 'spec', 'changes')
const codeReviewDir = join(root, 'spec', 'code-review')
const jobsDir = join(root, 'spec', 'jobs')
const tpl = join(root, 'scripts', 'templates', 'job.md')

const args = process.argv.slice(2)
const force = args.includes('--force')
const pos = args.filter((a) => !a.startsWith('--'))
const kind = pos[0]
const nnArg = pos[1]
const titleOverride = pos.slice(2).join(' ').trim()

if (!['plan', 'change', 'refactor'].includes(kind) || !nnArg) {
  console.error('Usage: pnpm spec:job <plan|change|refactor> <NN> ["job title"] [--force]')
  process.exit(1)
}

const nn = String(nnArg)
  .replace(/[^0-9]/g, '')
  .padStart(2, '0')
const srcDir = kind === 'plan' ? planDir : kind === 'change' ? changesDir : codeReviewDir
const srcLabel = kind === 'plan' ? 'plan' : kind === 'change' ? 'changes' : 'code-review'
const srcFile = readdirSync(srcDir).find((f) => f.startsWith(nn + '.') && f.endsWith('.md'))
if (!srcFile) {
  console.error(`No ${kind} at spec/${srcLabel}/${nn}.*.md`)
  process.exit(1)
}

const srcText = readFileSync(join(srcDir, srcFile), 'utf8')
const sourceRef = `${srcLabel}/${srcFile.replace(/\.md$/, '')}`
const type = kind === 'plan' ? 'new' : kind === 'change' ? 'change' : 'refactor'

let planRef = kind === 'refactor' ? 'none' : sourceRef
let title = titleOverride || frontmatterTitle(srcText) || h1Title(srcText)

if (kind === 'change') {
  planRef = unquote((srcText.match(/^plan:\s*(.+)$/m) || [])[1]?.trim()) || 'plan/UNKNOWN'
  title = titleOverride || frontmatterTitle(srcText) || title
}

const jobnn = nextNum(jobsDir)
const out = join(jobsDir, `${jobnn}.${slugify(title)}.md`)
if (existsSync(out) && !force) {
  console.error(`${out} exists; use --force to overwrite`)
  process.exit(1)
}

let doc = fill(readFileSync(tpl, 'utf8'), {
  JOB: jobnn,
  TYPE: type,
  SOURCE: sourceRef,
  PLAN: planRef,
  TITLE: title,
})

const acc = extractAcceptance(srcText)
if (acc) doc = doc.replace(/- \[ \] A1 .*/, acc)

writeFileSync(out, doc, 'utf8')
console.log(
  `Created spec/jobs/${jobnn}.${slugify(title)}.md  (type=${type}, source=${sourceRef}, plan=${planRef})`,
)
console.log(
  type === 'refactor'
    ? `Next: /create-refactor-job ${nn} fills job ${jobnn}, then /implement-job ${jobnn} implements it.`
    : `Next: /implement-job ${jobnn} fills the implementation checklist, builds, verifies, and reflects to SSOT.`,
)

function frontmatterTitle(t) {
  return unquote((t.match(/^title:\s*(.+)$/m) || [])[1]?.trim()) || null
}

// Frontmatter scalars are quoted in the scaffold templates so a placeholder cannot be read as
// YAML syntax. A consumer reading them back gets the quotes too, and they would land in a job's
// filename and its own frontmatter.
function unquote(value) {
  if (!value) return value
  const m = value.match(/^(['"])(.*)\1$/)
  return m ? m[2] : value
}

function h1Title(t) {
  const m = t.match(/^#\s+(.+)$/m)
  if (!m) return 'untitled'
  return m[1]
    .replace(/^Refactor Report\s+\d+:\s*/i, '')
    .replace(/^Job\s+\d+:\s*/i, '')
    .replace(/^\d+\.\s*/, '')
    .replace(/\s*\(as-built\)\s*$/i, '')
    .trim()
}

function extractAcceptance(t) {
  const section = sections(t).find(({ heading }) => {
    const normalized = heading.toLowerCase()
    return normalized.includes('acceptance') || heading.includes('수용 기준')
  })
  if (!section) return null

  // One item per top-level list entry; indented continuation lines and sub-bullets fold into their item.
  const items = []
  for (const line of section.body.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('<!--')) continue
    const bullet = line.match(/^(?:\d+\.|[-*])\s+(.*)$/)
    if (bullet) items.push(bullet[1].trim())
    else if (items.length)
      items[items.length - 1] += ' ' + trimmed.replace(/^(?:\d+\.|[-*])\s+/, '')
  }

  return items.length ? items.map((it, i) => `- [ ] A${i + 1} ${it}`).join('\n') : null
}

function sections(t) {
  const result = []
  // Lookahead alternatives: next `##` heading, a thematic-break line, or absolute end of input.
  // `$` alone would match every line end under the m flag and truncate the body to its first line.
  const re = /^##\s+(.+?)\s*\n([\s\S]*?)(?=^##\s+|^---\s*$|(?![\s\S]))/gm
  for (const m of t.matchAll(re)) result.push({ heading: m[1], body: m[2] })
  return result
}

// Monotonic across live + archive/: done docs move to archive/ but their numbers must never be reused.
function nextNum(dir) {
  const archive = join(dir, 'archive')
  const files = existsSync(archive)
    ? [...readdirSync(dir), ...readdirSync(archive)]
    : readdirSync(dir)
  const nums = files
    .map((f) => parseInt((f.match(/^(\d+)/) || [])[1], 10))
    .filter((n) => !Number.isNaN(n))
  return String((nums.length ? Math.max(...nums) : 0) + 1).padStart(2, '0')
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
