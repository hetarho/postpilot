#!/usr/bin/env node
// Scaffold the next sequential plan doc. Usage: node new-plan.mjs "<title>"  (pnpm spec:plan "<title>")
// /create-plan interviews the user, then lays down spec/plan/NN.slug.md from the template + fills it.
import { readFileSync, writeFileSync, readdirSync, existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve, join } from 'node:path'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const planDir = join(root, 'spec', 'plan')
const tpl = join(root, 'scripts', 'templates', 'plan.md')

const title = process.argv
  .slice(2)
  .filter((a) => !a.startsWith('--'))
  .join(' ')
  .trim()
if (!title) {
  console.error('Usage: pnpm spec:plan "<title>"')
  process.exit(1)
}

const nn = nextNum(planDir)
const out = join(planDir, `${nn}.${slugify(title)}.md`)
if (existsSync(out)) {
  console.error(`${out} exists`)
  process.exit(1)
}
writeFileSync(out, fill(readFileSync(tpl, 'utf8'), { NN: nn, TITLE: title }), 'utf8')
console.log(`Created spec/plan/${nn}.${slugify(title)}.md`)
console.log(
  'Next: /create-plan fills it via interview (purpose · scope · design · acceptance criteria + the policy/tech it needs), then adds it to 00.overview.',
)

function nextNum(dir) {
  const m = readdirSync(dir)
    .map((f) => parseInt((f.match(/^(\d+)/) || [])[1], 10))
    .filter((n) => !isNaN(n))
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
