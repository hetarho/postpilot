#!/usr/bin/env node
// Spec & workflow hygiene keeps executable documentation and the job board internally consistent.
//
//   A  Workflow-skill links resolve. Anchor fragments and {{placeholders}} are skipped; only the file part of a
//      relative link is checked. Scaffold-template links are relative to the generated destination, not the template.
//   B  Job status ↔ location consistency. A `status: done` job belongs under spec/jobs/archive/; a
//      `status: todo|doing` job belongs under spec/jobs/. This keeps the board trustworthy.
//   C  Scaffold templates substitute cleanly, so `pnpm spec:*` cannot emit a doc with a live placeholder in it.
//      `--probe` runs C's self-check alone and proves it bites.

import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { repoRoot, section, ok, note, fail } from './lib.mjs'

const problems = []
const probeMode = process.argv.includes('--probe')

section(
  probeMode
    ? 'Spec & workflow hygiene — scaffold-substitution probe'
    : 'Spec & workflow hygiene — doc links (R003) + job status/location (R004) + scaffolds',
)

// ---- C) scaffold templates substitute cleanly (defined first; the probe dispatches off it) ----
// Every `pnpm spec:*` renders a template by literal `{{KEY}}` replacement, so a template whose
// placeholder is not in that exact shape ships verbatim into a real document's frontmatter — where
// nothing downstream validates it.
//
// A frontmatter placeholder must therefore stay QUOTED (`key: '{{X}}'`). Unquoted, `key: {{X}}` is a
// nested flow mapping in YAML, so Prettier reformats its spacing to `{ { X } }` — and since
// `format:check` runs inside `pnpm lint`, the reformatted shape becomes the one the gate demands while
// `fill()` matches nothing. Quoting is what makes the substituting form and the Prettier-stable form
// the same form.
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

// The probe is the whole run when asked for: it proves C bites, and has nothing to say about A or B.
if (probeMode) probeScaffolds()

// ---- A) workflow-skill links resolve ----
const SKILL_ROOTS = ['.claude/skills', '.codex/skills']
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
const LINK = /\]\(([^)]+)\)/g
let checkedLinks = 0
for (const root of SKILL_ROOTS) {
  for (const file of skillFiles(join(repoRoot, root))) {
    const text = readFileSync(file, 'utf8')
    for (const m of text.matchAll(LINK)) {
      let target = m[1].trim()
      if (!target || target.includes('{{')) continue // template placeholder
      if (/^(https?:|mailto:|#)/.test(target)) continue // external / pure anchor
      target = target.split('#')[0] // drop anchor fragment
      if (!target) continue
      checkedLinks++
      const resolved = resolve(dirname(file), target)
      if (!existsSync(resolved)) {
        problems.push(
          `${file.replace(repoRoot + '/', '')} → broken link \`${m[1]}\` (resolves to a path that doesn't exist).`,
        )
      }
    }
  }
}
note(`checked ${checkedLinks} relative links across ${SKILL_ROOTS.join(', ')} (SKILL.md only)`)

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
    `${problems.length} spec-hygiene issue(s). See code-review 03 (R003 doc links · R004 job status/location) and code-review 11 (R002 scaffold placeholders).`,
  )
}
ok('workflow-doc links resolve; job status matches location; scaffolds substitute cleanly')

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
  process.exit(0)
}
