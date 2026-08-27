#!/usr/bin/env node
// Scaffold the next code-quality / code-review report.
// Usage: node new-code-review.mjs "<title>"  (pnpm spec:code-review "<title>")
// /create-code-review performs the read-only audit, then fills spec/code-review/NN.slug.md.
import { readFileSync, writeFileSync, readdirSync, existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve, join } from 'node:path'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const reportDir = join(root, 'spec', 'code-review')
const tpl = join(root, 'scripts', 'templates', 'code-review.md')

const title = process.argv
  .slice(2)
  .filter((a) => !a.startsWith('--'))
  .join(' ')
  .trim()
if (!title) {
  console.error('Usage: pnpm spec:code-review "<title>"')
  process.exit(1)
}

const nn = nextNum(reportDir)
const slug = slugify(title)
const out = join(reportDir, `${nn}.${slug}.md`)
if (existsSync(out)) {
  console.error(`${out} exists`)
  process.exit(1)
}

writeFileSync(
  out,
  fill(readFileSync(tpl, 'utf8'), { NN: nn, TITLE: title, DATE: localDate() }),
  'utf8',
)
console.log(`Created spec/code-review/${nn}.${slug}.md`)
console.log(
  `Next: /create-code-review fills the read-only report, then /create-refactor-job ${nn} creates an implementation job from selected findings.`,
)

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

function localDate() {
  const d = new Date()
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}
