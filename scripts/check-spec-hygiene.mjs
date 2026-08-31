#!/usr/bin/env node
// Spec & workflow hygiene keeps executable documentation and the job board internally consistent.
//
//   A  Workflow and spec links resolve. Anchor fragments and {{placeholders}} are skipped; only the file part of a
//      relative link is checked. Scaffold-template links are relative to the generated destination, not the template.
//   B  Job status ↔ location consistency. A `status: done` job belongs under spec/jobs/archive/; a
//      `status: todo|doing` job belongs under spec/jobs/. This keeps the board trustworthy.
//   C  Scaffold templates substitute cleanly, so `pnpm spec:*` cannot emit a doc with a live placeholder in it.
//      `--probe` runs A and C's self-checks and proves both guards bite without rejecting documentation examples.

import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { repoRoot, section, ok, note, fail } from './lib.mjs'

const problems = []
const probeMode = process.argv.includes('--probe')

section(
  probeMode
    ? 'Spec & workflow hygiene — link + scaffold-substitution probes'
    : 'Spec & workflow hygiene — doc links + job status/location + scaffolds',
)

// ---- C) scaffold templates substitute cleanly (defined first; the probe dispatches off it) ----
// Every `pnpm spec:*` renders a template by literal `{{KEY}}` replacement, so a template whose
// placeholder is not in that exact shape ships verbatim into a real document's frontmatter — where
// nothing downstream validates it.
//
// A frontmatter placeholder must therefore stay QUOTED (`key: '{{X}}'`). Unquoted, `key: {{X}}` is a
// nested flow mapping in YAML, so any formatter or editor that normalizes YAML respaces it to
// `{ { X } }` and `fill()` then matches nothing. Quoting makes the substituting form and the
// formatter-stable form the same form.
//
// No formatter gate stands behind this rule: `format:check` is scoped to the frontend workspace and
// these templates live in scripts/templates/, so this check is the only thing enforcing it.
const SCAFFOLDS = [
  { kind: 'plan', template: 'plan.md', script: 'new-plan.mjs' },
  { kind: 'change', template: 'change.md', script: 'new-change.mjs' },
  { kind: 'code-review', template: 'code-review.md', script: 'new-code-review.mjs' },
  { kind: 'job', template: 'job.md', script: 'new-job.mjs' },
]
const PLACEHOLDER = /\{\{([A-Z_]+)\}\}/g
const SPACED_BRACE = /\{\s+\{\s*[A-Z_]+\s*\}\s+\}/

// The keys are read out of the scaffold itself rather than restated here, so this check cannot drift
// from what the script actually supplies. Returns null when the call can't be found — the caller
// fails closed rather than reporting a pass it did not verify.
function scaffoldKeys(scriptText) {
  const at = scriptText.indexOf('fill(readFileSync')
  if (at === -1) return null
  const open = scriptText.indexOf('{', at)
  if (open === -1) return null
  let depth = 0
  let end = -1
  for (let i = open; i < scriptText.length; i++) {
    if (scriptText[i] === '{') depth++
    else if (scriptText[i] === '}' && --depth === 0) {
      end = i
      break
    }
  }
  if (end === -1) return null
  // Anchored to a key position (after `{` or `,`) so an uppercase identifier appearing inside a value
  // expression — a ternary's branches, say — cannot be mistaken for a key and quietly widen the set.
  const keyAt = /(?:[{,])\s*([A-Z_]+)\s*:/g
  const keys = [...scriptText.slice(open, end).matchAll(keyAt)].map((m) => m[1])
  return keys.length ? new Set(keys) : null
}

function scaffoldProblems({ kind, template, script }, templateText, scriptText) {
  const found = []
  if (SPACED_BRACE.test(templateText)) {
    found.push(
      `scripts/templates/${template} has a spaced-brace placeholder — a formatter rewrote an unquoted \`{{X}}\`, so fill() no longer substitutes it. Quote it: \`key: '{{X}}'\`.`,
    )
  }
  const keys = scaffoldKeys(scriptText)
  if (!keys) {
    found.push(
      `scripts/${script} — could not read its fill() key set, so a scaffolded ${kind} cannot be checked.`,
    )
    return found
  }
  // Render it the way the scaffold does, then let anything placeholder-shaped that survived speak.
  const rendered = [...keys].reduce(
    (acc, key) => acc.replaceAll(`{{${key}}}`, `probe-${key.toLowerCase()}`),
    templateText,
  )
  for (const [, name] of rendered.matchAll(PLACEHOLDER)) {
    found.push(
      `scripts/templates/${template} uses {{${name}}}, which scripts/${script} never supplies — it would ship verbatim in a scaffolded ${kind}.`,
    )
  }
  return found
}

const readScaffold = (scaffold) => ({
  templateText: readFileSync(join(repoRoot, 'scripts/templates', scaffold.template), 'utf8'),
  scriptText: readFileSync(join(repoRoot, 'scripts', scaffold.script), 'utf8'),
})

// ---- A) workflow and spec links resolve ----
const SKILL_ROOTS = ['.claude/skills', '.codex/skills']
const ROOT_DOCS = ['PRD.md', 'DEPLOY.md', 'README.md']
const skillFiles = (absDir) => {
  const out = []
  const walk = (d) => {
    for (const e of readdirSync(d, { withFileTypes: true })) {
      const p = join(d, e.name)
      if (e.isDirectory()) walk(p)
      else if (e.name === 'SKILL.md') out.push(p) // the workflow instruction docs (not vendored READMEs)
    }
  }
  if (existsSync(absDir)) walk(absDir)
  return out
}
const markdownFiles = (absDir) => {
  const out = []
  const walk = (d) => {
    for (const e of readdirSync(d, { withFileTypes: true })) {
      const p = join(d, e.name)
      if (e.isDirectory()) walk(p)
      else if (e.name.endsWith('.md')) out.push(p)
    }
  }
  if (existsSync(absDir)) walk(absDir)
  return out
}

const blankCode = (text) => text.replace(/[^\n]/g, ' ')

function withoutFencedCode(text) {
  let fence = null
  return text
    .split('\n')
    .map((line) => {
      if (fence) {
        const closing = line.match(/^ {0,3}(`{3,}|~{3,})[\t ]*$/)?.[1]
        if (closing && closing[0] === fence[0] && closing.length >= fence.length) fence = null
        return blankCode(line)
      }
      const opening = line.match(/^ {0,3}(`{3,}|~{3,})(.*)$/)
      const marker = opening?.[1]
      if (marker && (marker[0] === '~' || !opening[2].includes('`'))) {
        fence = marker
        return blankCode(line)
      }
      return line
    })
    .join('\n')
}

const tickRunLength = (text, at) => {
  let end = at
  while (text[end] === '`') end++
  return end - at
}

// spec/code-review/01 intentionally quotes a broken link as evidence. Markdown code spans and fenced blocks are
// examples, not navigation, so treating their link-shaped text as live would make the checker reject its own docs.
function withoutMarkdownCode(text) {
  const masked = [...withoutFencedCode(text)]
  for (let at = 0; at < masked.length; at++) {
    if (masked[at] !== '`') continue
    const width = tickRunLength(masked, at)
    let close = at + width
    while (close < masked.length) {
      if (masked[close] !== '`') {
        close++
        continue
      }
      const candidateWidth = tickRunLength(masked, close)
      if (candidateWidth === width) break
      close += candidateWidth
    }
    if (close >= masked.length) {
      at += width - 1
      continue
    }
    for (let i = at; i < close + width; i++) {
      if (masked[i] !== '\n') masked[i] = ' '
    }
    at = close + width - 1
  }
  return masked.join('')
}

const LINK = /\]\(([^)]+)\)/g
function linkDestination(raw) {
  const value = raw.trim()
  if (value.startsWith('<')) {
    const close = value.indexOf('>')
    if (close !== -1) return { target: value.slice(1, close), angleWrapped: true }
  }
  return { target: value.match(/^\S+/)?.[0] ?? '', angleWrapped: false }
}

function linkProblems(file, text, pathExists = existsSync) {
  const found = []
  let checked = 0
  for (const m of withoutMarkdownCode(text).matchAll(LINK)) {
    const destination = linkDestination(m[1])
    let target = destination.target
    if (!target || target.includes('{{')) continue // template placeholder
    if (/^(https?:|mailto:|#)/.test(target)) continue // external / pure anchor
    // plan/08 and archived job 13 use `](<file>)` to illustrate export syntax. That named placeholder is not a
    // repository path; other angle-wrapped destinations are valid Markdown links and stay checked.
    if (destination.angleWrapped && target === 'file') continue
    target = target.split('#')[0] // drop anchor fragment
    if (!target) continue
    checked++
    if (!pathExists(resolve(dirname(file), target))) {
      found.push(
        `${relative(repoRoot, file)} → broken link \`${m[1]}\` (resolves to a path that doesn't exist).`,
      )
    }
  }
  return { checked, problems: found }
}

// The probe is the whole run when asked for: A and C must both prove they reject corruption before the real gate runs.
if (probeMode) runProbes()

const linkFiles = [
  ...SKILL_ROOTS.flatMap((root) => skillFiles(join(repoRoot, root))),
  ...markdownFiles(join(repoRoot, 'spec')),
  ...ROOT_DOCS.map((file) => join(repoRoot, file)).filter(existsSync),
]
let checkedLinks = 0
for (const file of linkFiles) {
  const result = linkProblems(file, readFileSync(file, 'utf8'))
  checkedLinks += result.checked
  problems.push(...result.problems)
}
note(
  `checked ${checkedLinks} relative links across spec/**, ${ROOT_DOCS.join(', ')}, and ${SKILL_ROOTS.join(', ')} (SKILL.md only)`,
)

// ---- B) job status ↔ location ----
const readStatus = (file) => {
  const m = readFileSync(file, 'utf8').match(/^status:\s*([A-Za-z]+)/m)
  return m ? m[1].toLowerCase() : null
}
const jobsDir = join(repoRoot, 'spec/jobs')
if (existsSync(jobsDir)) {
  for (const e of readdirSync(jobsDir, { withFileTypes: true })) {
    if (!e.isFile() || !e.name.endsWith('.md')) continue // skip the archive/ dir
    const status = readStatus(join(jobsDir, e.name))
    if (status === 'done') {
      problems.push(
        `spec/jobs/${e.name} is \`status: done\` but still in spec/jobs/ — move it to spec/jobs/archive/ (a done job must be archived).`,
      )
    }
  }
  const archiveDir = join(jobsDir, 'archive')
  if (existsSync(archiveDir)) {
    for (const e of readdirSync(archiveDir, { withFileTypes: true })) {
      if (!e.isFile() || !e.name.endsWith('.md')) continue
      const status = readStatus(join(archiveDir, e.name))
      if (status === 'todo' || status === 'doing') {
        problems.push(
          `spec/jobs/archive/${e.name} is \`status: ${status}\` but archived — unfinished jobs belong in spec/jobs/, not archive/.`,
        )
      }
    }
  }
}

// ---- C) run it ----
for (const scaffold of SCAFFOLDS) {
  const { templateText, scriptText } = readScaffold(scaffold)
  problems.push(...scaffoldProblems(scaffold, templateText, scriptText))
}
note(`rendered ${SCAFFOLDS.length} scaffold template(s) against their scaffold's key set`)

if (problems.length) {
  for (const p of problems) console.error(`  \x1b[31m✗\x1b[0m ${p}`)
  fail(
    `${problems.length} spec-hygiene issue(s). Every checked relative link must resolve; a job's ` +
      `status must match its directory (done → spec/jobs/archive/, todo|doing → spec/jobs/); and a scaffold ` +
      `template's frontmatter placeholders must stay quoted so \`fill()\` can substitute them.`,
  )
}
ok('workflow-doc links resolve; job status matches location; scaffolds substitute cleanly')

function runProbes() {
  probeLinks()
  probeScaffolds()
  process.exit(0)
}

// Proves A catches a broken navigation link while preserving the documentation examples it intentionally excludes.
function probeLinks() {
  const file = join(repoRoot, 'spec/__link-probe__.md')
  const brokenCases = [
    '[broken](./missing.md)',
    '[angle-wrapped](<./missing file.md>)',
    '[titled](./missing.md "title")',
  ].join('\n')
  const broken = linkProblems(file, brokenCases, () => false)
  if (broken.checked !== 3 || broken.problems.length !== 3) {
    fail('the doc-link check did not report every known-broken relative-link form.')
  }
  const titledExisting = linkProblems(file, '[titled](./existing.md "title")', (resolved) =>
    resolved === join(dirname(file), 'existing.md'),
  )
  if (titledExisting.checked !== 1 || titledExisting.problems.length !== 0) {
    fail('the doc-link check treated a Markdown link title as part of an existing path.')
  }
  const examples = [
    '`[quoted evidence](./missing.md)`',
    '`multiline evidence\n[quoted evidence](./missing.md)\ncontinues here`',
    '```md\n```not a closing fence\n[fenced example](./missing.md)\n```',
    '~~~md\n[tilde-fenced example](./missing.md)\n~~~',
    '![placeholder](<file>)',
  ].join('\n')
  const ignored = linkProblems(file, examples, () => false)
  if (ignored.checked !== 0 || ignored.problems.length !== 0) {
    fail('the doc-link check rejected a code example or angle-bracket placeholder.')
  }
  ok('doc-link check catches 3 relative-link forms, accepts link titles, and ignores code examples/placeholders')
}

// Proves C fails on the two shapes it exists to catch, using in-memory fixtures — a guard is only
// worth having once someone has watched it bite. It also asserts the real template passes, so the
// check cannot earn its green by rejecting everything.
function probeScaffolds() {
  const scaffold = SCAFFOLDS.find((s) => s.kind === 'code-review')
  const { templateText, scriptText } = readScaffold(scaffold)
  const cases = [
    {
      what: 'a formatter-spaced frontmatter placeholder',
      text: templateText.replace("title: '{{TITLE}}'", 'title: { { TITLE } }'),
    },
    {
      what: 'a placeholder the scaffold never supplies',
      text: templateText.replace('status: report', 'status: {{UNSUPPLIED}}'),
    },
  ]
  const missed = cases.filter((c) => scaffoldProblems(scaffold, c.text, scriptText).length === 0)
  if (missed.length) {
    for (const c of missed) console.error(`  \x1b[31m✗\x1b[0m not caught: ${c.what}`)
    fail(`${missed.length} scaffold corruption shape(s) slipped past the check.`)
  }
  if (scaffoldProblems(scaffold, templateText, scriptText).length > 0) {
    fail('the real code-review template is reported as broken — the check is too strict.')
  }
  ok(`scaffold check catches ${cases.length} corruption shape(s) and passes the real template`)
}
